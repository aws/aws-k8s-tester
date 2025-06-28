###############################################################################
# Base image, arguments, and environment
###############################################################################
ARG CUDA_MAJOR_VERSION=12
ARG CUDA_MINOR_VERSION=9

FROM nvidia/cuda:$CUDA_MAJOR_VERSION.$CUDA_MINOR_VERSION.0-devel-ubuntu22.04

ARG CUDA_MAJOR_VERSION
ARG CUDA_MINOR_VERSION

# Disable interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

###############################################################################
# System packages
###############################################################################
RUN apt update \
 && apt upgrade -y \
 && apt install -y --no-install-recommends \
       build-essential \
       ca-certificates \
       cmake \
       curl \
       emacs \
       git \
       jq \
       libopencv-dev \
       software-properties-common \
       wget \
       unzip \
       vim \
       pkg-config \
       gdb \
       lcov \
       libbz2-dev \
       zlib1g-dev \
       openssl \
       libssl-dev \
       libsqlite3-dev \
       libgdbm-dev \
       libc6-dev \
       libbz2-dev \
       libncurses-dev \
       tk-dev \
       libffi-dev \
       libcap-dev \
       gnupg2 \
       gpg-agent \
 && rm -rf /var/lib/apt/lists/*

###############################################################################
# Build and install Python from source
###############################################################################
ARG PYTHON=python3.10
ARG PYTHON_VERSION=3.10.12

RUN curl -sL https://www.python.org/ftp/python/$PYTHON_VERSION/Python-$PYTHON_VERSION.tgz | tar xvz -C /tmp \
 && cd /tmp/Python-$PYTHON_VERSION \
 && ./configure --enable-shared --prefix=/usr/local \
 && make -j$(nproc) \
 && make install \
 && cd && rm -rf /tmp/Python-$PYTHON_VERSION

RUN ln -s /usr/local/bin/pip3 /usr/bin/pip \
 && ln -s /usr/local/bin/$PYTHON /usr/local/bin/python \
 && pip3 --no-cache-dir install --upgrade pip setuptools

###############################################################################
# Install Pytorch from Source
###############################################################################
ARG PYTORCH_BRANCH=v2.6.0
ARG PYTORCH_BUILD_ENV="MAX_JOBS=8 BUILD_TEST=0"

# envs needed to make the path of NVCC known to the compilation
ENV CUDA_HOME=/usr/local/cuda
ENV LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/usr/local/cuda/lib64:/usr/local/cuda/extras/CUPTI/lib64
ENV PATH=$PATH:$CUDA_HOME/bin
ENV TORCH_CUDA_ARCH_LIST="7.5 8.0 8.6 8.7 8.9 9.0 10.0"

RUN pip3 install typing-extensions sympy pyyaml
RUN git clone https://github.com/pytorch/pytorch.git /tmp/pytorch \
      --recursive \
      --branch $PYTORCH_BRANCH \
 && cd /tmp/pytorch \
 && eval "$PYTORCH_BUILD_ENV python3 setup.py install" \
 && cd && rm -rf /tmp/pytorch

###############################################################################
# Application files and Python dependencies
###############################################################################
WORKDIR /app
COPY infer.py /app/
COPY requirements.txt /app/
RUN pip install --no-cache-dir -r requirements.txt
