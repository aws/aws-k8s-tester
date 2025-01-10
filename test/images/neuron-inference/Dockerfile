###############################################################################
# 0) Base image, arguments, and environment
###############################################################################
FROM ubuntu:22.04

# Disable interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Ensure Python prints are unbuffered so we see logs in real time
ENV PYTHONUNBUFFERED=1

# Neuron SDK components version numbers
ARG NEURONX_CC_VERSION=2.15.143.0
ARG NEURONX_FRAMEWORK_VERSION=2.1.2.2.3.1
ARG NEURONX_COLLECTIVES_LIB_VERSION=2.22.33.0-d2128d1aa
ARG NEURONX_RUNTIME_LIB_VERSION=2.22.19.0-5856c0b42
ARG NEURONX_TOOLS_VERSION=2.19.0.0

# Python
ARG PYTHON=python3.10
ARG PYTHON_VERSION=3.10.12

# General Python environment
ENV PYTHONDONTWRITEBYTECODE=1 \
    LANG=C.UTF-8 \
    LC_ALL=C.UTF-8

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
 && pip --no-cache-dir install --upgrade pip setuptools

###############################################################################
# 3) AWS Neuron packages
###############################################################################
RUN . /etc/os-release \
 && echo "deb https://apt.repos.neuron.amazonaws.com $VERSION_CODENAME main" \
    > /etc/apt/sources.list.d/neuron.list \
 && wget -qO - https://apt.repos.neuron.amazonaws.com/GPG-PUB-KEY-AMAZON-AWS-NEURON.PUB | apt-key add - \
 && apt-get update \
 && apt-get install -y --no-install-recommends \
    aws-neuronx-tools=$NEURONX_TOOLS_VERSION \
    aws-neuronx-collectives=$NEURONX_COLLECTIVES_LIB_VERSION \
    aws-neuronx-runtime-lib=$NEURONX_RUNTIME_LIB_VERSION \
 && rm -rf /var/lib/apt/lists/* \
 && apt-get clean

###############################################################################
# 4) Install PyTorch Neuron, Transformers Neuron, etc. via pip
###############################################################################
RUN pip install --no-cache-dir \
    torch-neuronx==$NEURONX_FRAMEWORK_VERSION \
    neuronx-cc==$NEURONX_CC_VERSION \
    transformers-neuronx

###############################################################################
# 5) Application files and Python dependencies
###############################################################################
WORKDIR /app
COPY infer.py /app/

# Optionally install extra dependencies if needed, e.g.:
# RUN pip install --no-cache-dir ...

# Default command
CMD ["python", "/app/infer.py"]
