package prom

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/storage"
	"github.com/toolkits/pkg/logger"
	"go.uber.org/atomic"
)

type RemoteConfig struct {
	Name                string `json:"name"`
	Ka                  string `json:"ka"`
	Env                 string `json:"env"`
	Url                 string `json:"url"`
	RemoteTimeoutSecond int    `json:"remote_timeout_second"`
}

type Section struct {
	RemoteWrite []RemoteConfig `json:"remote_write"`
	RemoteRead  []RemoteConfig `json:"remote_read"`
}

type DataSource struct {
	Section   Section                  //配置
	PushQueue chan []prompb.TimeSeries // 数据推送的chan
	// LocalTmpDir  string                          // 本地临时目录，存放queries.active文件
	Queryable    storage.SampleAndChunkQueryable // 除了promql的查询，需要后端存储，如查询series
	QueryEngine  *promql.Engine                  // promql相关查询
	WriteTargets []*HttpClient                   // remote_write写入的后端地址
}

type HttpClient struct {
	remoteName string // Used to differentiate clients in metrics.
	url        *url.URL
	Client     *http.Client
	timeout    time.Duration
}

type safePromQLNoStepSubqueryInterval struct {
	value atomic.Int64
}

func durationToInt64Millis(d time.Duration) int64 {
	return int64(d / time.Millisecond)
}
func (i *safePromQLNoStepSubqueryInterval) Set(ev model.Duration) {
	i.value.Store(durationToInt64Millis(time.Duration(ev)))
}
func (i *safePromQLNoStepSubqueryInterval) Get(int64) int64 {
	return i.value.Load()
}

func NewPromDataSource(cg Section) *DataSource {
	pd := &DataSource{
		Section:   cg,
		PushQueue: make(chan []prompb.TimeSeries, 10000),
	}
	return pd
}

func NewMetricPoint(metric string, tags map[string]string, time int64, value float64) *MetricPoint {
	return &MetricPoint{
		Metric:  metric,
		TagsMap: tags,
		Time:    time,
		Value:   value,
	}
}

type MetricPoint struct {
	Metric  string            `json:"metric"` // 指标名称
	TagsMap map[string]string `json:"tags"`   // 数据标签
	Time    int64             `json:"time"`   // 时间戳，单位是秒
	Value   float64           `json:"value"`  // 内部字段，最终转换之后的float64数值
}

func (pd *DataSource) Init() error {
	//// 模拟创建本地存储目录
	//dbDir, err := ioutil.TempDir("", "tsdb-api-ready")
	//if err != nil {
	//	logger.Errorf("[error_create_local_tsdb_dir][err: %v]", err)
	//	return err
	//}
	//pd.LocalTmpDir = dbDir
	//promlogConfig := promlog.Config{}
	//// 使用本地目录创建remote-storage
	//remoteS := remote.NewStorage(promlog.New(&promlogConfig), prometheus.DefaultRegisterer, func() (int64, error) {
	//	return 0, nil
	//}, dbDir, 1*time.Minute, nil)
	//
	//// ApplyConfig 加载queryables
	//remoteReadC := make([]*pc.RemoteReadConfig, 0)
	//for _, u := range pd.Section.RemoteRead {
	//	logger.Infof("u: %v", u)
	//	ur, err := url.Parse(u.Url)
	//	if err != nil {
	//		logger.Errorf("[prome_ds_init_error][parse_url_error][url:%+v][err:%+v]", u.Url, err)
	//		continue
	//	}
	//
	//	remoteReadC = append(remoteReadC,
	//		&pc.RemoteReadConfig{
	//			URL:           &config_util.URL{URL: ur},
	//			RemoteTimeout: model.Duration(time.Duration(u.RemoteTimeoutSecond) * time.Second),
	//			ReadRecent:    true,
	//		},
	//	)
	//}
	//if len(remoteReadC) == 0 {
	//	logger.Errorf("[prome_ds_error_got_zero_remote_read_storage]")
	//	return fmt.Errorf("[prome_ds_error_got_zero_remote_read_storage]")
	//}
	//err = remoteS.ApplyConfig(&pc.Config{RemoteReadConfigs: remoteReadC})
	//if err != nil {
	//	logger.Errorf("[error_load_remote_read_config][err: %v]", err)
	//	return err
	//}
	//pLogger := log.NewNopLogger()
	//
	//noStepSubqueryInterval := &safePromQLNoStepSubqueryInterval{}
	//
	//queryQueueDir, err := ioutil.TempDir(dbDir, "prom_query_concurrency")
	//opts := promql.EngineOpts{
	//	Logger:                   log.With(pLogger, "component", "query engine"),
	//	Reg:                      prometheus.DefaultRegisterer,
	//	MaxSamples:               50000000,
	//	Timeout:                  30 * time.Second,
	//	ActiveQueryTracker:       promql.NewActiveQueryTracker(queryQueueDir, 20, log.With(pLogger, "component", "activeQueryTracker")),
	//	LookbackDelta:            5 * time.Minute,
	//	NoStepSubqueryIntervalFn: noStepSubqueryInterval.Get,
	//	EnableAtModifier:         true,
	//}
	//
	//queryEngine := promql.NewEngine(opts)
	//pd.QueryEngine = queryEngine
	//pd.Queryable = remoteS

	// 初始化writeClients
	if len(pd.Section.RemoteWrite) == 0 {
		logger.Warningf("[prome_ds_init_with_zero_RemoteWrite_target]")
		logger.Infof("[successfully_init_prometheus_datasource][remote_read_num:%+v][remote_write_num:%+v]",
			len(pd.Section.RemoteRead),
			len(pd.Section.RemoteWrite),
		)
		return fmt.Errorf("[prome_ds_init_with_zero_RemoteWrite_target]")
	}
	writeTs := make([]*HttpClient, 0)
	for _, u := range pd.Section.RemoteWrite {
		ur, err := url.Parse(u.Url)
		if err != nil {
			logger.Errorf("[prome_ds_init_error][parse_url_error][url:%+v][err:%+v]", u.Url, err)
			continue
		}
		writeTs = append(writeTs,
			&HttpClient{
				remoteName: u.Name,
				url:        ur,
				Client:     &http.Client{},
				timeout:    time.Duration(u.RemoteTimeoutSecond) * time.Second,
			})
	}
	pd.WriteTargets = writeTs
	// 开启prometheus 队列消费协程
	go pd.remoteWrite()
	logger.Infof("[successfully_init_prometheus_datasource][remote_read_num:%+v][remote_write_num:%+v]",
		0,
		len(writeTs),
	)
	return nil
}
