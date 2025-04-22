###############################################################################
# 0) Base image, arguments, and environment
###############################################################################
FROM nvidia/cuda:12.8.0-devel-ubuntu22.04

# Disable interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Python build args
ARG PYTHON=python3.10
ARG PYTHON_VERSION=3.10.12

# General Python environment
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    LANG=C.UTF-8 \
    LC_ALL=C.UTF-8

ARG PYTORCH_BRANCH=v2.6.0
ARG PYTORCH_BUILD_ENV="MAX_JOBS=8 BUILD_TEST=0"

###############################################################################
# 1) System packages
###############################################################################
RUN apt-get update \
 && apt-get upgrade -y \
 && apt-get install -y --no-install-recommends \
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
 && rm -rf /var/lib/apt/lists/* \
 && apt-get clean

###############################################################################
# 2) Build and install Python from source
###############################################################################
RUN wget -q https://www.python.org/ftp/python/$PYTHON_VERSION/Python-$PYTHON_VERSION.tgz \
 && tar -xzf Python-$PYTHON_VERSION.tgz \
 && cd Python-$PYTHON_VERSION \
 && ./configure --enable-shared --prefix=/usr/local \
 && make -j"$(nproc)" \
 && make install \
 && cd .. && rm -rf Python-$PYTHON_VERSION* \
 && ln -s /usr/local/bin/pip3 /usr/bin/pip \
 && ln -s /usr/local/bin/$PYTHON /usr/local/bin/python \
 && pip3 --no-cache-dir install --upgrade pip setuptools

###############################################################################
# 3) Application files and Python dependencies
###############################################################################
WORKDIR /app
COPY infer.py /app/
COPY requirements.txt /app/
RUN pip install --no-cache-dir -r requirements.txt

###############################################################################
# 4) Install Pytorch from Source
###############################################################################
# envs needed to make the path of NVCC known to the compilation
ENV CUDA_HOME=/usr/local/cuda
ENV LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/usr/local/cuda/lib64:/usr/local/cuda/extras/CUPTI/lib64
ENV PATH=$PATH:$CUDA_HOME/bin
# this list could be minimized based on the supported GPUs
ENV TORCH_CUDA_ARCH_LIST="7.5 8.0 8.6 8.7 8.9 9.0 10.0"

RUN pip3 install typing-extensions sympy
RUN git clone \
      --recursive https://github.com/pytorch/pytorch.git \
      --branch $PYTORCH_BRANCH \
 && cd pytorch && eval "$PYTORCH_BUILD_ENV python3 setup.py install" && cd .. \
 && rm -rf pytorch
