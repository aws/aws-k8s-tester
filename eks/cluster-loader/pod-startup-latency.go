package clusterloader

import (
	"encoding/json"
	"fmt"
	"os"

	measurement_util "k8s.io/perf-tests/clusterloader2/pkg/measurement/util"
)

func ParsePodStartupLatency(fpath string) (perfData measurement_util.PerfData, err error) {
	rf, err := os.OpenFile(fpath, os.O_RDONLY, 0444)
	if err != nil {
		return measurement_util.PerfData{}, fmt.Errorf("failed to open %q (%v)", fpath, err)
	}
	defer rf.Close()
	err = json.NewDecoder(rf).Decode(&perfData)
	return perfData, err
}

func MergePodStartupLatency(datas ...measurement_util.PerfData) (perfData measurement_util.PerfData) {
	if len(datas) == 0 {
		return perfData
	}
	if len(datas) == 1 {
		return datas[0]
	}

	perfData.Labels = make(map[string]string)
	labelToUnit := make(map[string]string)
	labelToData := make(map[string]map[string]float64)

	for _, d := range datas {
		perfData.Version = d.Version
		for k, v := range d.Labels {
			perfData.Labels[k] = v
		}
		for _, cur := range d.DataItems {
			b, err := json.Marshal(cur.Labels)
			if err != nil {
				panic(err)
			}
			key := string(b)

			labelToUnit[key] = cur.Unit
			prev, ok := labelToData[key]
			if ok {
				for k, v := range prev {
					// average
					cur.Data[k] += v
					cur.Data[k] /= 2.0
				}
			}
			labelToData[key] = cur.Data
		}
	}

	for key, data := range labelToData {
		unit := labelToUnit[key]
		var labels map[string]string
		if err := json.Unmarshal([]byte(key), &labels); err != nil {
			panic(err)
		}
		perfData.DataItems = append(perfData.DataItems, measurement_util.DataItem{
			Data:   data,
			Labels: labels,
			Unit:   unit,
		})
	}
	return perfData
}
