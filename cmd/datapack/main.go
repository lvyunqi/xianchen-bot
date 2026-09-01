package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/appinfo"
	"xianlv/internal/config"
	"xianlv/internal/handler"
	"xianlv/internal/model"
	"xianlv/internal/service"
	"xianlv/internal/storage"
)

const packageName = "仙尘全套游戏数据包-v" + appinfo.Version

func main() {
	root, err := os.Getwd()
	must(err)
	buildRoot := filepath.Join(root, "build")
	target := filepath.Join(buildRoot, packageName)
	absBuild, _ := filepath.Abs(buildRoot)
	absTarget, _ := filepath.Abs(target)
	if !strings.HasPrefix(strings.ToLower(absTarget), strings.ToLower(absBuild)+string(os.PathSeparator)) {
		must(fmt.Errorf("数据包目标目录不安全: %s", absTarget))
	}
	staging := false
	if err := os.RemoveAll(target); err != nil {
		staging = true
		target = filepath.Join(buildRoot, "."+packageName+"-staging-"+fmt.Sprintf("%d", time.Now().UnixNano()))
		fmt.Fprintln(os.Stderr, "现有解压目录正在使用，改用临时目录生成最终ZIP:", err)
	}
	if staging {
		defer func() { _ = os.RemoveAll(target) }()
	}
	must(os.MkdirAll(filepath.Join(target, "plugin_data", "仙尘", "data"), 0o755))
	must(os.MkdirAll(filepath.Join(target, "plugin_data", "仙尘", "数据模板"), 0o755))
	must(os.MkdirAll(filepath.Join(target, "plugin_data", "仙尘", "uploads"), 0o755))
	must(os.MkdirAll(filepath.Join(target, "plugin_data", "仙尘", "images"), 0o755))

	dataRoot := filepath.Join(target, "plugin_data", "仙尘")
	store, err := storage.Open(config.Runtime(dataRoot))
	must(err)
	must(service.SeedPlayerCommandMenus(store))
	counts := exportTemplates(store.DB, filepath.Join(dataRoot, "数据模板"))
	must(writeJSON(filepath.Join(target, "257项无前缀指令清单.json"), handler.CommandTable))
	must(writeJSON(filepath.Join(target, "附加无前缀指令清单.json"), handler.AuxiliaryCommands()))
	must(writeJSON(filepath.Join(target, "神令权限清单.json"), service.GMCommandCatalog()))
	must(writeJSON(filepath.Join(target, "数据数量清单.json"), counts))
	must(store.Close())

	copyFile(filepath.Join(root, "build", "仙尘.dll"), filepath.Join(target, "仙尘.dll"))
	copyFile(filepath.Join(root, "README.md"), filepath.Join(target, "README.md"))
	copyFile(filepath.Join(root, "config.yaml"), filepath.Join(target, "config.yaml"))
	copyFile(filepath.Join(root, "web", "admin", "assets", "logo.png"), filepath.Join(dataRoot, "images", "logo.png"))
	must(os.WriteFile(filepath.Join(target, "安装说明.txt"), []byte(installText), 0o644))
	must(os.WriteFile(filepath.Join(dataRoot, "授权激活说明.txt"), []byte(licenseActivationText), 0o644))

	zipPath := filepath.Join(buildRoot, packageName+".zip")
	if err := os.Remove(zipPath); err != nil && !os.IsNotExist(err) {
		must(err)
	}
	must(zipDirectory(target, zipPath, packageName))
	fmt.Println(zipPath)
}

func exportTemplates(db *gorm.DB, dir string) map[string]int64 {
	resources := []struct {
		Name string
		Rows any
	}{
		{"系统参数", &[]model.SystemSetting{}}, {"境界配置", &[]model.Realm{}}, {"灵根图鉴", &[]model.SpiritualRootTemplate{}}, {"物品分类", &[]model.ItemCategory{}},
		{"稀有度配置", &[]model.Rarity{}}, {"物品数据", &[]model.Item{}}, {"掉落池", &[]model.DropPool{}},
		{"掉落项", &[]model.DropEntry{}}, {"事件数据", &[]model.Event{}}, {"任务数据", &[]model.TaskTemplate{}},
		{"功法数据", &[]model.Skill{}}, {"灵兽数据", &[]model.PetTemplate{}}, {"副本数据", &[]model.Dungeon{}}, {"竞技段位", &[]model.ArenaTier{}},
		{"丹方数据", &[]model.AlchemyRecipe{}}, {"器谱数据", &[]model.ArtifactTemplate{}}, {"称号数据", &[]model.Title{}},
		{"合成配方", &[]model.SynthesisRecipe{}},
		{"活动数据", &[]model.Activity{}}, {"邮件数据", &[]model.Mail{}}, {"签到配置", &[]model.CheckinReward{}},
		{"商店数据", &[]model.ShopEntry{}}, {"兑换码数据", &[]model.RedemptionCode{}}, {"公告数据", &[]model.Notice{}},
		{"菜单配置", &[]model.AdminMenu{}}, {"敏感词数据", &[]model.SensitiveWord{}},
		{"管理员设置", &[]model.ManagerAccount{}}, {"内容审核队列", &[]model.ContentReview{}}, {"慢查询记录", &[]model.SlowQueryLog{}},
		{"神令操作审计", &[]model.OperationLog{}},
		{"阵法配置", &[]model.FormationConfig{}}, {"符箓配置", &[]model.TalismanConfig{}}, {"傀儡配置", &[]model.PuppetConfig{}},
		{"秘境争夺配置", &[]model.SecretRealmConflictConfig{}}, {"传承配置", &[]model.InheritanceConfig{}}, {"悟道配置", &[]model.DaoInsightConfig{}},
		{"仙魔战场配置", &[]model.ImmortalDemonBattlefieldConfig{}}, {"灵根进化配置", &[]model.SpiritualRootEvolutionConfig{}},
		{"渡劫心魔配置", &[]model.InnerDemonConfig{}}, {"合体技配置", &[]model.CoupleCombinationSkillConfig{}},
		{"仙药培育配置", &[]model.ImmortalHerbConfig{}}, {"法宝炼化配置", &[]model.ArtifactRefinementConfig{}},
		{"天机推演配置", &[]model.DestinyDeductionConfig{}}, {"天地灵脉配置", &[]model.LeylineConfig{}},
		{"宗门战争配置", &[]model.SectWarConfig{}}, {"仙缘奇遇配置", &[]model.ImmortalEncounterConfig{}}, {"宇宙星河配置", &[]model.StarRealmConfig{}},
		{"地图数据", &[]model.WorldLocation{}}, {"修仙界灵脉", &[]model.WorldLeyline{}},
	}
	counts := make(map[string]int64, len(resources))
	for _, resource := range resources {
		must(db.Order("id").Find(resource.Rows).Error)
		must(writeJSON(filepath.Join(dir, resource.Name+".json"), resource.Rows))
		value := reflectSliceLength(resource.Rows)
		counts[resource.Name] = int64(value)
	}
	return counts
}

func reflectSliceLength(pointer any) int {
	data, err := json.Marshal(pointer)
	must(err)
	var rows []json.RawMessage
	must(json.Unmarshal(data, &rows))
	return len(rows)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func copyFile(source, target string) {
	in, err := os.Open(source)
	must(err)
	defer in.Close()
	out, err := os.Create(target)
	must(err)
	_, err = io.Copy(out, in)
	must(err)
	must(out.Close())
}

func zipDirectory(source, target, archiveRoot string) error {
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	archive := zip.NewWriter(file)
	var paths []string
	err = filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	if err == nil {
		sort.Strings(paths)
		for _, path := range paths {
			inside, _ := filepath.Rel(source, path)
			relative := filepath.Join(archiveRoot, inside)
			header, headerErr := zip.FileInfoHeader(mustStat(path))
			if headerErr != nil {
				err = headerErr
				break
			}
			header.Name = filepath.ToSlash(relative)
			header.Method = zip.Deflate
			writer, createErr := archive.CreateHeader(header)
			if createErr != nil {
				err = createErr
				break
			}
			input, openErr := os.Open(path)
			if openErr != nil {
				err = openErr
				break
			}
			_, copyErr := io.Copy(writer, input)
			_ = input.Close()
			if copyErr != nil {
				err = copyErr
				break
			}
		}
	}
	closeErr := archive.Close()
	fileErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return fileErr
}

func mustStat(path string) os.FileInfo { info, err := os.Stat(path); must(err); return info }
func must(err error) {
	if err != nil {
		panic(err)
	}
}

const installText = `仙尘 v` + appinfo.Version + `

插件名：仙尘
作者：随缘 · 夜空

1. 将“仙尘.dll”放入 Bee 插件目录并加载。
2. 首次启用未授权属于正常状态；直接点击插件“设置”打开授权窗口。
3. 授权窗口会显示机器码。将机器码发给作者随缘，由独立主人工具签发卡密。
4. 在授权窗口粘贴完整卡密并点击“验证并激活”，无需创建或编辑任何文件。
5. 激活成功后页面自动进入数据后台，默认使用 plugin_data/仙尘/data/xianlv.db。升级旧版时插件会自动继续读取旧数据目录，玩家数据不会丢失。
6. 如需云端授权吊销与PostgreSQL云数据，请由主人按源码 docs/云授权与云数据部署.md 设置；未配置云服务时不会假装已经上云。
7. 所有后台数据保存后立即生效。
8. 在系统参数填写 owner.user_id，在管理员设置填写管理ID、角色和启用状态。
9. 群内全部命令无前缀。首次发送：入道 道号。
10. 群回复和好友私信默认调用QQ开放平台原生Markdown，不使用自定义模板ID。
11. 原生Markdown失败时自动回退Bee普通文本消息，避免指令完全无回复。
12. 未入道用户除入道、获取ID、获取群ID外保持静默。
13. 神令只在群内执行；未授权账号发送神令时保持静默。角色权限依次为护法、长老、宗主、仙尊、道祖。
`

const licenseActivationText = `仙尘授权激活说明

首次启用插件后，点击插件“设置”打开授权窗口。
窗口会显示本机机器码；把机器码发给作者随缘，收到卡密后直接粘贴到窗口并点击激活。
窗口会自动保存授权并进入数据后台，不需要打开、创建或修改任何授权文件。

卡密绑定机器并带有效期；修改、截断、跨机器复制或过期都会拒绝加载并记录到 security.log。
卡密生成器及签名私钥不包含在玩家数据包和游戏后台中。
`
