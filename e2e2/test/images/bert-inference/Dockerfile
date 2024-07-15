# Use the NVIDIA CUDA runtime as a parent image
FROM nvidia/cuda:12.5.0-devel-ubuntu22.04

# Set environment variable to disable interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Dependency version numbers
ARG PYTHON=python3.10
ARG PYTHON_VERSION=3.10.12
ARG PIP=pip3

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

# Install Python
RUN wget -q https://www.python.org/ftp/python/$PYTHON_VERSION/Python-$PYTHON_VERSION.tgz \
 && tar -xzf Python-$PYTHON_VERSION.tgz \
 && cd Python-$PYTHON_VERSION \
 && ./configure --enable-shared --prefix=/usr/local \
 && make -j $(nproc) && make install \
 && cd .. && rm -rf ../Python-$PYTHON_VERSION* \
 && ln -s /usr/local/bin/pip3 /usr/bin/pip \
 && ln -s /usr/local/bin/$PYTHON /usr/local/bin/python \
 && ${PIP} --no-cache-dir install --upgrade \
    pip \
    setuptools

# Set the working directory in the container
WORKDIR /app

# Copy only the necessary files into the container at /app
COPY infer.py /app/
COPY requirements.txt /app/

# Install any needed packages specified in requirements.txt
RUN python -m pip install --upgrade pip && \
    pip install --no-cache-dir -r requirements.txt
