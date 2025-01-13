###############################################################################
# 0) Base image, arguments, and environment
###############################################################################
FROM ubuntu:22.04

# Disable interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Ensure Python prints are unbuffered so we see logs in real time
ENV PYTHONUNBUFFERED=1

# Neuron SDK components version numbers
ARG NEURONX_CC_VERSION=2.16.345.0
ARG NEURONX_FRAMEWORK_VERSION=2.5.1.2.4.0
ARG NEURONX_COLLECTIVES_LIB_VERSION=2.23.133.0-3e70920f2
ARG NEURONX_RUNTIME_LIB_VERSION=2.23.110.0-9b5179492
ARG NEURONX_TOOLS_VERSION=2.20.204.0

# Python
ARG PYTHON=python3.10
ARG PYTHON_VERSION=3.10.12

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PYTHONIOENCODING=UTF-8 \
    LANG=C.UTF-8 \
    LC_ALL=C.UTF-8

# Extend library paths for Neuron
ENV LD_LIBRARY_PATH="${LD_LIBRARY_PATH}:/opt/aws/neuron/lib"
ENV LD_LIBRARY_PATH="${LD_LIBRARY_PATH}:/usr/local/lib"
ENV PATH="/opt/aws/neuron/bin:${PATH}"

###############################################################################
# 1) Base system packages, user setup
###############################################################################
RUN apt-get update \
 && apt-get upgrade -y \
 && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    curl \
    git \
    jq \
    wget \
    unzip \
    vim \
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
    gnupg2 \
    gpg-agent \
    openssh-server \
    sudo \
 && rm -rf /var/lib/apt/lists/* \
 && apt-get clean

###############################################################################
# 2) Neuron SDK
###############################################################################
RUN . /etc/os-release \
 && echo "deb https://apt.repos.neuron.amazonaws.com focal main" > /etc/apt/sources.list.d/neuron.list \
 && wget -qO - https://apt.repos.neuron.amazonaws.com/GPG-PUB-KEY-AMAZON-AWS-NEURON.PUB | apt-key add - \
 && apt-get update \
 && apt-get install -y \
      aws-neuronx-tools=${NEURONX_TOOLS_VERSION} \
      aws-neuronx-collectives=${NEURONX_COLLECTIVES_LIB_VERSION} \
      aws-neuronx-runtime-lib=${NEURONX_RUNTIME_LIB_VERSION} \
 && rm -rf /var/lib/apt/lists/* \
 && apt-get clean

###############################################################################
# 3) Python 3.10 from source
###############################################################################
RUN wget -q https://www.python.org/ftp/python/${PYTHON_VERSION}/Python-${PYTHON_VERSION}.tgz \
 && tar -xzf Python-${PYTHON_VERSION}.tgz \
 && cd Python-${PYTHON_VERSION} \
 && ./configure --enable-shared --prefix=/usr/local \
 && make -j $(nproc) && make install \
 && cd .. && rm -rf Python-${PYTHON_VERSION}* \
 && ln -s /usr/local/bin/pip3 /usr/bin/pip \
 && ln -s /usr/local/bin/${PYTHON} /usr/local/bin/python \
 && pip --no-cache-dir install --upgrade pip setuptools wheel

###############################################################################
# 4) Install PyTorch Neuron, Transformers Neuron, etc. via pip
###############################################################################
RUN pip config set global.extra-index-url https://pip.repos.neuron.amazonaws.com \
 && pip install --force-reinstall \
      "torch-neuronx==${NEURONX_FRAMEWORK_VERSION}" \
      "neuronx-cc==${NEURONX_CC_VERSION}" \
      "transformers==4.36.2"

###############################################################################
# 5) Application files and Python dependencies
###############################################################################
WORKDIR /app
COPY infer.py /app/

# Default command
CMD ["python", "/app/infer.py"]
