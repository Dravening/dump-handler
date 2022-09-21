package prom

import (
	"context"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/storage"
	"github.com/toolkits/pkg/logger"
)

// 从promql中抽取标签matcher的函数
func parseMatchersParam(matchers []string) ([][]*labels.Matcher, error) {
	var matcherSets [][]*labels.Matcher
	for _, s := range matchers {
		matchers, err := parser.ParseMetricSelector(s)
		if err != nil {
			return nil, err
		}
		matcherSets = append(matcherSets, matchers)
	}

OUTER:
	for _, ms := range matcherSets {
		for _, lm := range ms {
			if lm != nil && !lm.Matches("") {
				continue OUTER
			}
		}
		return nil, errors.New("match[] must contain at least one non-empty matcher")
	}
	return matcherSets, nil
}

// 毫秒时间戳函数
func millisecondTs(t time.Time) int64 {
	return t.Unix()*1000 + int64(t.Nanosecond())/int64(time.Millisecond)
}

// 转行为utc时间
func timeParse(ts int64) time.Time {
	t := float64(ts)
	s, ns := math.Modf(t)
	ns = math.Round(ns*1000) / 1000
	return time.Unix(int64(s), int64(ns*float64(time.Second))).UTC()
}

func (pd *DataSource) CommonQuerySeries(qlStrFinal string) storage.SeriesSet {
	matcherSets, err := parseMatchersParam([]string{qlStrFinal})
	if err != nil {
		logger.Errorf("[prome_query_error][parse_label_match_error][err:%+v]", err)
		return nil
	}
	tEnd := time.Now().Unix()
	tStart := tEnd - 60*5

	startT := millisecondTs(timeParse(tStart))
	endT := millisecondTs(timeParse(tEnd))

	ctx, _ := context.WithTimeout(context.Background(), time.Second*30)
	q, err := pd.Queryable.Querier(ctx, startT, endT)
	if err != nil {
		logger.Errorf("[prome_query_error][get_querier_errro]")
		return nil
	}

	defer q.Close()

	hints := &storage.SelectHints{
		Start: startT,
		End:   endT,
		Func:  "series", // There is no series function, this token is used for lookups that don't need samples.
	}

	// Get all series which match matchers.
	s := q.Select(true, hints, matcherSets[0]...)

	return s

}

// 查询一个标签的值列表
// 对应prometheus 中的 /api/v1/label/<label_name>/values
func (pd *DataSource) QueryLabelValue(promql string, targetLabel string) []string {
	s := pd.CommonQuerySeries(promql)
	if s.Warnings() != nil {
		logger.Warningf("[prome_query_error][series_set_iter_error][warning:%+v]", s.Warnings())
	}

	if err := s.Err(); err != nil {
		logger.Errorf("[prome_query_error][series_set_iter_error][err:%+v]", err)
		return nil
	}

	var sets []storage.SeriesSet
	sets = append(sets, s)
	set := storage.NewMergeSeriesSet(sets, storage.ChainedSeriesMerge)
	labelValuesSet := make(map[string]struct{})
	thisSeriesNum := 0
	for set.Next() {
		series := set.At()
		thisSeriesNum++
		for _, lb := range series.Labels() {
			if lb.Name == targetLabel {
				labelValuesSet[lb.Value] = struct{}{}
			}
		}
	}
	vals := make([]string, len(labelValuesSet))
	i := 0
	for val := range labelValuesSet {
		vals[i] = val
		i++
	}

	sort.Strings(vals)
	logger.Infof("[QueryLabelValue][promql:%v][targetLabel:%v][values:%v]", promql, targetLabel, vals)
	return vals
}

func tsToUtcTs(s int64) time.Time {
	return time.Unix(s, 0).UTC()
}

// 查询数据
func (pd *DataSource) QueryData(qlStrFinal string) {

	tEnd := time.Now().Unix()
	tStart := tEnd - 60*5

	startT := tsToUtcTs(tStart)
	endT := tsToUtcTs(tEnd)

	resolution := time.Second * 15

	q, err := pd.QueryEngine.NewRangeQuery(pd.Queryable, qlStrFinal, startT, endT, resolution)
	if err != nil {
		logger.Errorf("[prome_query_error][QueryData_error_may_be_parse_ql_error][args:%+v][err:%+v]", qlStrFinal, err)
		return
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Second*30)
	res := q.Exec(ctx)
	if res.Err != nil {
		logger.Errorf("[prome_query_error][rangeQuery_exec_error][args:%+v][err:%+v]", qlStrFinal, res.Err)
		q.Close()
		return
	}
	mat, ok := res.Value.(promql.Matrix)
	if !ok {
		logger.Errorf("[promql.Engine.exec: invalid expression type %q]", res.Value.Type())
		q.Close()
		return
	}
	if res.Err != nil {
		logger.Errorf("[prome_query_error][res.Matrix_error][args:%+v][err:%+v]", qlStrFinal, res.Err)
		q.Close()
		return
	}
	for _, m := range mat {
		logger.Infof("[vector_res:%v]", m.Metric.String())
		for _, p := range m.Points {

			ts := time.Unix(p.T/1e3, 0).Format("2006-01-02 15:04:05")
			logger.Infof("[detail][ts:%v][value:%v]", ts, p.V)
		}

	}
	q.Close()

	return
}
