# Use Ubuntu 20.04 as the base image
FROM ubuntu:20.04

# Neuron SDK components versions
ARG NEURONX_FRAMEWORK_VERSION=2.11.0.0
ARG NEURONX_RUNTIME_LIB_VERSION=2.11.7.0
ARG NEURONX_TOOLS_VERSION=2.11.8.0
ARG NEURONX_CC_VERSION=2.11.8.0

# Set environment variables
ENV DEBIAN_FRONTEND=noninteractive
ENV PYTHONUNBUFFERED=1
ENV PYTHONIOENCODING=UTF-8
ENV LD_LIBRARY_PATH="/opt/aws/neuron/lib:/usr/local/lib"
ENV PATH="/opt/aws/neuron/bin:$PATH"

# Install system dependencies including libsqlite3-dev and libbz2-dev for Python
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    curl \
    wget \
    zlib1g-dev \
    gnupg2 \
    libssl-dev \
    libffi-dev \
    libsqlite3-dev \
    libbz2-dev \
    libopenblas-dev \
    libomp5 \
    && rm -rf /var/lib/apt/lists/*

# Add Neuron repository and install Neuron SDK components
RUN echo "deb https://apt.repos.neuron.amazonaws.com focal main" > /etc/apt/sources.list.d/neuron.list && \
    wget -qO - https://apt.repos.neuron.amazonaws.com/GPG-PUB-KEY-AMAZON-NEURON.PUB | apt-key add - && \
    apt-get update && \
    apt-get install -y \
    aws-neuronx-tools=${NEURONX_TOOLS_VERSION} \
    aws-neuronx-runtime-lib=${NEURONX_RUNTIME_LIB_VERSION} \
    && rm -rf /var/lib/apt/lists/*

# Install Python 3.10 with sqlite3 and bz2 support
RUN wget -q https://www.python.org/ftp/python/3.10.12/Python-3.10.12.tgz && \
    tar -xzf Python-3.10.12.tgz && \
    cd Python-3.10.12 && \
    ./configure --enable-shared --enable-optimizations --with-ensurepip=install && \
    make -j $(nproc) && make install && \
    cd .. && rm -rf Python-3.10.12*

# Upgrade pip and install required Python packages
RUN python3.10 -m pip install --upgrade pip

# Install Neuron-related Python packages from the Neuron repository
RUN python3.10 -m pip install --no-cache-dir \
    --extra-index-url https://pip.repos.neuron.amazonaws.com \
    torch-neuronx==${NEURONX_FRAMEWORK_VERSION} \
    torch-xla==1.13.* \
    torchvision

# Install additional Python packages
RUN python3.10 -m pip install --no-cache-dir \
    transformers==4.29 \
    numpy==1.23 \
    pynvml

# Set the working directory
WORKDIR /app

# Copy training and inference scripts
COPY train_bert_neuron.py /app/train_bert_neuron.py
COPY infer_bert_neuron.py /app/infer_bert_neuron.py

