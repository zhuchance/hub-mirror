package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/spf13/pflag"
)

var (
	rawImages  = pflag.StringP("images", "", "", "镜像列表（每行一个镜像地址）")
	maxImages  = pflag.IntP("maxImages", "", 20, "镜像个数限制")
	registry   = pflag.StringP("registry", "", "registry.cn-hangzhou.aliyuncs.com", "目标仓库地址")
	namespace  = pflag.StringP("namespace", "", "mgfw", "目标命名空间")
	username   = pflag.StringP("username", "", "", "仓库用户名")
	password   = pflag.StringP("password", "", "", "仓库密码")
	outputPath = pflag.StringP("outputPath", "", "output.sh", "Shell 脚本输出路径")
	reportPath = pflag.StringP("reportPath", "", "report.md", "Markdown 报告输出路径")
)

// SyncResult 表示单个镜像的同步结果
type SyncResult struct {
	Source     string
	Target     string
	SourceHash string
	TargetHash string
	ShortHash  string
	Status     string
	Error      string
}

func main() {
	pflag.Parse()

	// 1. 解析镜像列表
	imageList := parseImages(*rawImages)
	if len(imageList) == 0 {
		panic("未找到有效的镜像地址")
	}
	if len(imageList) > *maxImages {
		panic(fmt.Sprintf("镜像个数 %d 超过限制 %d", len(imageList), *maxImages))
	}
	fmt.Printf("共 %d 个镜像待同步\n", len(imageList))

	// 2. 连接 Docker
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	// 3. 登录仓库
	if *username == "" || *password == "" {
		panic("username or password cannot be empty.")
	}
	authConfig := types.AuthConfig{
		Username:      *username,
		Password:      *password,
		ServerAddress: *registry,
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		panic(err)
	}
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)
	_, err = cli.RegistryLogin(context.Background(), authConfig)
	if err != nil {
		panic(err)
	}

	// 4. 逐个同步镜像（顺序执行，单个失败不影响后续镜像）
	results := make([]SyncResult, 0, len(imageList))
	for _, source := range imageList {
		result := syncImage(cli, source, *registry, *namespace, authStr)
		results = append(results, result)
		printResult(result)
	}

	// 5. 生成 Markdown 报告
	if err := generateReport(results, *reportPath); err != nil {
		panic(err)
	}

	// 6. 生成 Shell 脚本输出
	if err := generateOutput(results, *outputPath); err != nil {
		panic(err)
	}

	fmt.Printf("\n同步完成！报告已输出至 %s，脚本已输出至 %s\n", *reportPath, *outputPath)
}

// parseImages 从纯文本中解析镜像列表
// 规则：每行一个镜像，去除 \r 和反引号，跳过空行和含空格的行
func parseImages(raw string) []string {
	images := make([]string, 0)
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		// 清理：去除回车符、反引号、首尾空白
		line = strings.ReplaceAll(line, "\r", "")
		line = strings.ReplaceAll(line, "`", "")
		line = strings.TrimSpace(line)
		// 跳过空行和含空格的行（注释等）
		if line == "" || strings.Contains(line, " ") {
			continue
		}
		images = append(images, line)
	}
	return images
}

// getTargetImage 构造目标镜像地址
// 例: gcr.io/google-samples/microservices-demo/emailservice:v0.3.5
//  => registry.cn-hangzhou.aliyuncs.com/mgfw/emailservice:v0.3.5
func getTargetImage(source, registry, namespace string) string {
	parts := strings.Split(source, "/")
	imageNameTag := parts[len(parts)-1]
	return fmt.Sprintf("%s/%s/%s", registry, namespace, imageNameTag)
}

// syncImage 执行单个镜像的拉取、打标签、推送和 Hash 校验
func syncImage(cli *client.Client, source, registry, namespace, authStr string) SyncResult {
	target := getTargetImage(source, registry, namespace)
	result := SyncResult{
		Source: source,
		Target: target,
	}
	ctx := context.Background()

	fmt.Printf("🚀 开始同步: [%s] => [%s]\n", source, target)

	// 1. 拉取源镜像
	pullOut, err := cli.ImagePull(ctx, source, types.ImagePullOptions{})
	if err != nil {
		result.Status = "pull_failed"
		result.Error = err.Error()
		return result
	}
	io.Copy(os.Stdout, pullOut)
	pullOut.Close()

	// 2. 获取源镜像 Hash
	srcInspect, _, err := cli.ImageInspectWithRaw(ctx, source)
	if err != nil {
		result.Status = "inspect_failed"
		result.Error = err.Error()
		return result
	}
	result.SourceHash = srcInspect.ID
	result.ShortHash = shortHash(srcInspect.ID)

	// 3. 重新打标签
	err = cli.ImageTag(ctx, source, target)
	if err != nil {
		result.Status = "tag_failed"
		result.Error = err.Error()
		return result
	}

	// 4. 推送目标镜像
	pushOut, err := cli.ImagePush(ctx, target, types.ImagePushOptions{
		RegistryAuth: authStr,
	})
	if err != nil {
		result.Status = "push_failed"
		result.Error = err.Error()
		return result
	}
	io.Copy(os.Stdout, pushOut)
	pushOut.Close()

	// 5. 获取目标镜像 Hash
	tgtInspect, _, err := cli.ImageInspectWithRaw(ctx, target)
	if err != nil {
		result.Status = "inspect_target_failed"
		result.Error = err.Error()
		return result
	}
	result.TargetHash = tgtInspect.ID

	// 6. 比对 Hash
	if result.SourceHash == result.TargetHash {
		result.Status = "success"
	} else {
		result.Status = "hash_mismatch"
	}

	return result
}

// shortHash 从完整镜像 ID 中提取短 Hash
// 例: sha256:a657... => a657...
func shortHash(fullHash string) string {
	if strings.HasPrefix(fullHash, "sha256:") {
		h := fullHash[7:]
		if len(h) > 12 {
			return h[:12]
		}
		return h
	}
	return fullHash
}

// printResult 打印单个镜像的同步结果
func printResult(r SyncResult) {
	fmt.Println(strings.Repeat("-", 50))
	switch r.Status {
	case "success":
		fmt.Printf("✅ Hash 校验一致！镜像成功推送至: %s\n", r.Target)
		fmt.Printf("   源镜像 ID:   %s\n", r.SourceHash)
		fmt.Printf("   目标镜像 ID: %s\n", r.TargetHash)
	case "hash_mismatch":
		fmt.Printf("⚠️ Hash 不一致！请检查镜像\n")
		fmt.Printf("   源镜像 ID:   %s\n", r.SourceHash)
		fmt.Printf("   目标镜像 ID: %s\n", r.TargetHash)
	default:
		fmt.Printf("❌ 同步失败 [%s]: %s\n", r.Status, r.Error)
	}
}

// generateReport 生成 Markdown 报告（用于 GITHUB_STEP_SUMMARY）
func generateReport(results []SyncResult, path string) error {
	var sb strings.Builder
	sb.WriteString("### 📦 镜像同步与 Hash 校验报告\n\n")
	sb.WriteString("| 源镜像 | 目标镜像 | Image ID (Hash) | 校验结果 |\n")
	sb.WriteString("| :--- | :--- | :--- | :--- |\n")
	for _, r := range results {
		short := r.ShortHash
		if short == "" {
			short = "-"
		}
		var statusIcon string
		switch r.Status {
		case "success":
			statusIcon = "✅ 一致"
		case "hash_mismatch":
			statusIcon = "⚠️ 不一致"
		case "push_failed":
			statusIcon = "❌ 推送失败"
		case "pull_failed":
			statusIcon = "❌ 拉取失败"
		default:
			statusIcon = fmt.Sprintf("❌ %s", r.Status)
		}
		sb.WriteString(fmt.Sprintf("| `%s` | `%s` | `%s` | %s |\n", r.Source, r.Target, short, statusIcon))
	}
	return os.WriteFile(path, []byte(sb.String()), 0644)
}

// generateOutput 生成 Shell 脚本（用于 Issue 评论，供用户本地拉取使用）
func generateOutput(results []SyncResult, path string) error {
	tmpl, err := template.New("pull_images").Parse(`{{- range . -}}

docker pull {{ .Target }}
docker tag {{ .Target }} {{ .Source }}

{{ end -}}`)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	// 仅包含同步成功的镜像
	successResults := make([]SyncResult, 0)
	for _, r := range results {
		if r.Status == "success" {
			successResults = append(successResults, r)
		}
	}
	if len(successResults) == 0 {
		return nil
	}
	return tmpl.Execute(f, successResults)
}
