package plugins

const wrk = `
cd ${HOME} \
  && git clone https://github.com/wg/wrk.git \
  && pushd wrk \
  && make all \
  && sudo cp ./wrk /usr/local/bin/wrk \
  && popd \
  && rm -rf ./wrk \
  && wrk --version || true && which wrk

`
