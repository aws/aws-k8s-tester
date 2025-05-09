ARG CUDA_MAJOR_VERSION=12
ARG CUDA_MINOR_VERSION=8

# Use the NVIDIA CUDA runtime as a parent image
FROM nvidia/cuda:$CUDA_MAJOR_VERSION.$CUDA_MINOR_VERSION.0-devel-ubuntu22.04

# Set environment variable to disable interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Set default values for MASTER_ADDR, MASTER_PORT, and NUM_GPUS_PER_NODE
ENV MASTER_ADDR=127.0.0.1
ENV MASTER_PORT=12355

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
 && rm -rf /var/lib/apt/lists/*

# Install Python
ARG PYTHON=python3.10
ARG PYTHON_VERSION=3.10.12

RUN curl -sL https://www.python.org/ftp/python/$PYTHON_VERSION/Python-$PYTHON_VERSION.tgz | tar xvz -C /tmp \
 && cd /tmp/Python-$PYTHON_VERSION \
 && ./configure --enable-shared --prefix=/usr/local \
 && make -j $(nproc) \
 && make install \
 && cd && rm -rf /tmp/Python-$PYTHON_VERSION

RUN ln -s /usr/local/bin/pip3 /usr/bin/pip \
 && ln -s /usr/local/bin/$PYTHON /usr/local/bin/python \
 && pip --no-cache-dir install --upgrade pip setuptools

# Install Pytorch from Source
ARG PYTORCH_BRANCH=v2.6.0
ARG PYTORCH_BUILD_ENV="MAX_JOBS=8 BUILD_TEST=0"

ENV CUDA_HOME=/usr/local/cuda
ENV LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/usr/local/cuda/lib64:/usr/local/cuda/extras/CUPTI/lib64
ENV PATH=$PATH:$CUDA_HOME/bin
ENV TORCH_CUDA_ARCH_LIST="7.5 8.0 8.6 8.7 8.9 9.0 10.0"

RUN pip install typing-extensions sympy pyyaml
RUN git clone https://github.com/pytorch/pytorch.git /tmp/pytorch \
        --recursive \
        --branch $PYTORCH_BRANCH \
 && cd /tmp/pytorch \
 && eval "$PYTORCH_BUILD_ENV python3 setup.py install" \
 && cd && rm -rf /tmp/pytorch

RUN apt-get update -y && \
    apt-get remove -y --allow-change-held-packages \
    libmlx5-1 ibverbs-utils libibverbs-dev libibverbs1 libnccl2 libnccl-dev && \
    rm -rf /opt/hpcx /usr/local/mpi /usr/local/ucx /etc/ld.so.conf.d/hpcx.conf

RUN apt-get install -y --allow-unauthenticated \
    sudo git gcc vim kmod openssh-client openssh-server build-essential \
    wget curl autoconf libtool gdb automake python3-distutils cmake \
    apt-utils devscripts debhelper libsubunit-dev check pkg-config libhwloc-dev

RUN ldconfig

# SSH configuration
RUN mkdir -p /var/run/sshd && \
    sed -i 's/[ #]\(.*StrictHostKeyChecking \).*/ \1no/g' /etc/ssh/ssh_config && \
    echo "    UserKnownHostsFile /dev/null" >> /etc/ssh/ssh_config && \
    sed -i 's/#\(StrictModes \).*/\1no/g' /etc/ssh/sshd_config

# Set environment variables for OpenMPI, CUDA, EFA, and NCCL
ENV LD_LIBRARY_PATH /opt/amazon/openmpi/lib64:/opt/amazon/openmpi/lib:/opt/amazon/efa/lib64:/opt/aws-ofi-nccl/install/lib:/usr/local/cuda/lib64:/usr/local/cuda/extras/CUPTI/lib64:/usr/local/lib/:/usr/lib64:/usr/lib/x86_64-linux-gnu/:/usr/lib/aarch64-linux-gnu/:$LD_LIBRARY_PATH
ENV PATH /usr/local/cuda/bin:/opt/amazon/openmpi/bin:/opt/amazon/efa/bin:/usr/sbin:/usr/bin:/usr/local/bin:$PATH

# Install EFA
ARG EFA_INSTALLER_VERSION=latest
RUN curl -sL https://efa-installer.amazonaws.com/aws-efa-installer-$EFA_INSTALLER_VERSION.tar.gz | tar xvz -C /tmp \
 && cd /tmp/aws-efa-installer \
 && ./efa_installer.sh -y -g -d --skip-kmod --skip-limit-conf --no-verify \
 && cd && rm -rf /tmp/aws-efa-installer

# Install NCCL
ARG LIBNCCL_VERSION=2.26.2-1
RUN apt update \
 && apt install -y \
        libnccl2=$LIBNCCL_VERSION+cuda$CUDA_MAJOR_VERSION.$CUDA_MINOR_VERSION \
        libnccl-dev=$LIBNCCL_VERSION+cuda$CUDA_MAJOR_VERSION.$CUDA_MINOR_VERSION \
 && rm -rf /var/lib/apt/lists/* \

# Install AWS-OFI-NCCL plugin
ARG AWS_OFI_NCCL_VERSION=1.14.2
RUN curl -sL https://github.com/aws/aws-ofi-nccl/releases/download/v$AWS_OFI_NCCL_VERSION/aws-ofi-nccl-$AWS_OFI_NCCL_VERSION.tar.gz | tar xvz -C /tmp \
 && cd /tmp/aws-ofi-nccl-$AWS_OFI_NCCL_VERSION \
 && ./configure \
        --prefix=/opt/aws-ofi-nccl/install \
        --with-mpi=/opt/amazon/openmpi \
        --with-libfabric=/opt/amazon/efa \
        --with-cuda=/usr/local/cuda \
        --enable-platform-aws \
        --disable-tests \
 && make -j $(nproc) \
 && make install \
 && cd && rm -rf /tmp/aws-ofi-nccl-$AWS_OFI_NCCL_VERSION

ENV NCCL_PROTO simple
ENV LD_LIBRARY_PATH=/usr/local/cuda/lib64:$LD_LIBRARY_PATH

RUN rm -rf /var/lib/apt/lists/*

# Set the working directory in the container
WORKDIR /app

# Copy the training script and install requirements
COPY train.py /app/
COPY requirements.txt /app/
RUN pip install --no-cache-dir -r requirements.txt
