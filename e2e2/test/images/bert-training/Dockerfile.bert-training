# Use the NVIDIA CUDA runtime as a parent image
FROM nvidia/cuda:12.5.0-devel-ubuntu22.04

# Set environment variable to disable interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Set default values for MASTER_ADDR and MASTER_PORT
ENV MASTER_ADDR=127.0.0.1
ENV MASTER_PORT=12355

# Install Python 3.11
RUN apt-get update && apt-get install -y \
    software-properties-common && \
    add-apt-repository ppa:deadsnakes/ppa && \
    apt-get update && \
    apt-get install -y \
    python3.11 \
    python3.11-dev \
    python3.11-distutils \
    python3-pip && \
    rm -rf /var/lib/apt/lists/*

# Create a symbolic link to use python3.11 as python
RUN ln -sf /usr/bin/python3.11 /usr/bin/python

# Set the working directory in the container
WORKDIR /app

# Copy only the necessary files into the container at /app
COPY train.py /app/
COPY requirements.txt /app/

# Install any needed packages specified in requirements.txt
RUN python -m pip install --upgrade pip && \
    pip install --no-cache-dir -r requirements.txt

ARG EFA_INSTALLER_VERSION=latest
# 1.7.4+ is required, to enforce proper EFA function with OFI_NCCL_DISABLE_GDR_REQUIRED_CHECK=0
ARG AWS_OFI_NCCL_VERSION=1.9.1
ARG NCCL_TESTS_VERSION=master

# Install necessary dependencies and remove old ones
RUN apt-get update -y && \
    apt-get remove -y --allow-change-held-packages \
    libmlx5-1 ibverbs-utils libibverbs-dev libibverbs1 libnccl2 libnccl-dev && \
    rm -rf /opt/hpcx /usr/local/mpi /usr/local/ucx /etc/ld.so.conf.d/hpcx.conf && \
    ldconfig && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --allow-unauthenticated \
    sudo git gcc vim kmod openssh-client openssh-server build-essential \
    wget curl autoconf libtool gdb automake python3-distutils cmake \
    apt-utils devscripts debhelper libsubunit-dev check pkg-config libhwloc-dev

# SSH configuration
RUN mkdir -p /var/run/sshd && \
    sed -i 's/[ #]\(.*StrictHostKeyChecking \).*/ \1no/g' /etc/ssh/ssh_config && \
    echo "    UserKnownHostsFile /dev/null" >> /etc/ssh/ssh_config && \
    sed -i 's/#\(StrictModes \).*/\1no/g' /etc/ssh/sshd_config

# Set environment variables for OpenMPI and CUDA
ENV LD_LIBRARY_PATH /opt/amazon/openmpi/lib64:/opt/amazon/openmpi/lib:/opt/amazon/efa/lib64:/opt/aws-ofi-nccl/install/lib:/usr/local/cuda/lib64:/usr/local/cuda/extras/CUPTI/lib64:/usr/local/lib/:/usr/lib64:/usr/lib/x86_64-linux-gnu/:$LD_LIBRARY_PATH
ENV PATH /usr/local/cuda/bin:/opt/amazon/openmpi/bin:/opt/amazon/efa/bin:/usr/sbin:/usr/bin:/usr/local/bin:$PATH

# Install EFA
RUN cd $HOME \
    && curl -O https://efa-installer.amazonaws.com/aws-efa-installer-${EFA_INSTALLER_VERSION}.tar.gz \
    && tar -xf $HOME/aws-efa-installer-${EFA_INSTALLER_VERSION}.tar.gz \
    && cd aws-efa-installer \
    && ./efa_installer.sh -y -g -d --skip-kmod --skip-limit-conf --no-verify \
    && rm -rf $HOME/aws-efa-installer

# Install NCCL
RUN apt-key del 7fa2af80 && \
    curl -L -O https://developer.download.nvidia.com/compute/cuda/repos/ubuntu1804/x86_64/cuda-keyring_1.0-1_all.deb && \
    dpkg -i cuda-keyring_1.0-1_all.deb && \
    sudo apt install libnccl2=2.18.5-1+cuda12.2 libnccl-dev=2.18.5-1+cuda12.2

# Install AWS-OFI-NCCL plugin
RUN export OPAL_PREFIX="" && \
    git clone https://github.com/aws/aws-ofi-nccl.git /opt/aws-ofi-nccl && \
    cd /opt/aws-ofi-nccl && \
    git checkout v${AWS_OFI_NCCL_VERSION}-aws && \
    ./autogen.sh && \
    ./configure --prefix=/opt/aws-ofi-nccl/install --with-libfabric=/opt/amazon/efa/ --with-cuda=/usr/local/cuda --with-mpi=/opt/amazon/openmpi/ && \
    make && make install

# Set default values for MASTER_ADDR and MASTER_PORT for local testing
ENV MASTER_ADDR=127.0.0.1
ENV MASTER_PORT=12355

# Set environment variables for NCCL and clean up
ENV NCCL_PROTO simple
RUN rm -rf /var/lib/apt/lists/*
# Ensure NCCL library is found first
ENV LD_LIBRARY_PATH=/usr/local/cuda/lib64:$LD_LIBRARY_PATH
