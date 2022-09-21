package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"dump-handler/logic"
	"dump-handler/thirdparty/cos"
	"dump-handler/thirdparty/prom"

	"github.com/toolkits/pkg/logger"
)

var (
	// 普罗
	promUrl string //普罗地址
	env     string //部署环境
	ka      string //租户
	// cos
	cosUrl    string //OSS url
	secretID  string //OSS secret_id
	secretKey string //OSS secret_key

	podId        string //PodId
	postfix      string //文件名的时间后缀
	locaFilename string //OOM DumpFile
)

func init() {
	// prom
	flag.StringVar(&promUrl, "prom", "10.150.30.6:9090", "promUrl")
	flag.StringVar(&env, "e", "test", "ENV")
	flag.StringVar(&ka, "ka", "default", "KA")
	// oss
	// cosurl: <BucketName-APPID>.cos.<Region>.myqcloud.com   注意这里包含了存储桶
	flag.StringVar(&cosUrl, "cosurl", "https://release-retail-1253467224.cos.ap-beijing.myqcloud.com", "ossUrl")
	flag.StringVar(&secretID, "secret", "AKIDc2C5h38xVnnX0C7mBI9j24pJXVCdngJe", "ossSecretID")
	flag.StringVar(&secretKey, "secretkey", "6vTmoa5JGwMVMxHK6bqoWyB9rJVR5ScW", "ossSecretKey")
	//
	flag.StringVar(&locaFilename, "filepath", "/dumps/oom", "maybe the path is 'dumps/oom'?")
	flag.StringVar(&podId, "k", "ops", "PodId")
	postfix = time.Now().Format("20060102150405")
}

// 判断所给路径文件是否存在
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func main() {
	flag.Parse()
	remoteConfig := []prom.RemoteConfig{
		{
			Name:                "prometheus",
			Ka:                  ka,
			Env:                 env,
			Url:                 fmt.Sprintf("http://%s/api/v1/write", promUrl),
			RemoteTimeoutSecond: 5,
		},
	}
	promConfig := prom.Section{RemoteWrite: remoteConfig}
	pd := prom.NewPromDataSource(promConfig)
	if err := pd.Init(); err != nil {
		return
	}

	// 判断dump文件是否存在
	exist, err := PathExists(locaFilename)
	if err != nil {
		logger.Error("get dir error![%v]\n", err)
		return
	}
	if exist {
		fileName := fmt.Sprintf("%s/%s/jvm/%s-%s", ka, env, podId, postfix)
		f, err := os.Open(locaFilename)
		if err != nil {
			logger.Error("open file error![%v]\n", err)
			return
		}
		if err = cos.Upload(cosUrl, secretID, secretKey, fileName, f); err != nil {
			logger.Error("upload file error![%v]\n", err)
			return
		}
		if err := logic.AlarmToProm(pd, cosUrl, fileName, ka, env); err != nil {
			logger.Error("send alarm to prom failed,[%v]\n", err)
			return
		}
	}
}
