package logic

import (
	"fmt"
	"time"

	"dump-handler/thirdparty/prom"

	"github.com/toolkits/pkg/logger"
)

const (
	BIZ_OOM_DUMP = "biz_oom_dump"
)

func newOOMDumpTags(cosUrl, fileName, ka, env string) map[string]string {
	return map[string]string{
		"cos_url":   cosUrl,
		"file_name": fileName,
		"trace_id":  fmt.Sprintf("%s/%s", cosUrl, fileName),
		"ka":        ka,
		"env":       env,
	}
}

func AlarmToProm(pd *prom.DataSource, cosUrl, fileName, ka, env string) error {
	tags := newOOMDumpTags(cosUrl, fileName, ka, env)

	alarm := prom.NewMetricPoint(
		BIZ_OOM_DUMP,
		tags,
		time.Now().Unix(),
		float64(1))

	if err := pd.RemoteWrite([]prom.MetricPoint{
		*alarm,
	}); err != nil {
		logger.Error(err)
	}
	return nil
}
