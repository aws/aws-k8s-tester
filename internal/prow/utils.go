package prow

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.uber.org/zap"
)

func mergeStrings(ss1, ss2 []string) (ss []string) {
	all := make(map[string]struct{})
	for _, v := range ss1 {
		all[v] = struct{}{}
	}
	for _, v := range ss2 {
		all[v] = struct{}{}
	}
	ss = make([]string, 0, len(all))
	for k := range all {
		ss = append(ss, k)
	}
	sort.Strings(ss)
	return ss
}

func copyStrings(ss []string) (cs []string) {
	cs = make([]string, len(ss))
	copy(cs, ss)
	return cs
}

func unzip(lg *zap.Logger, p string, targetDir string) error {
	lg.Info("unzipping", zap.String("zip", p), zap.String("target-dir", targetDir))
	r, err := zip.OpenReader(p)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		fpath := filepath.Join(targetDir, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(targetDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal path %q", fpath) // ZipSlip http://bit.ly/2MsjAWE
		}
		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(fpath, os.ModePerm); err != nil {
				return err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}
		of, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		_, err = io.Copy(of, rc)
		of.Close()
		if err != nil {
			return err
		}
	}
	lg.Info("unzipped", zap.String("zip", p), zap.String("target-dir", targetDir))
	return nil
}

func exist(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
