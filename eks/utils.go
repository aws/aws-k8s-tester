package eks

import (
	"fmt"
	"os"
	"sync"

	"go.uber.org/zap"
)

func catchInterrupt(lg *zap.Logger, stopc chan struct{}, once *sync.Once, sigc chan os.Signal, run func() error) (err error) {
	errc := make(chan error)
	go func() {
		errc <- run()
	}()
	select {
	case _, ok := <-stopc:
		rerr := <-errc
		lg.Info("interrupted", zap.Error(rerr))
		err = fmt.Errorf("stopc returned, stopc open %v, run function returned %v", ok, rerr)
	case sig := <-sigc:
		once.Do(func() { close(stopc) })
		rerr := <-errc
		err = fmt.Errorf("received os signal %v, closed stopc, run function returned %v", sig, rerr)
	case err = <-errc:
	}
	return err
}
