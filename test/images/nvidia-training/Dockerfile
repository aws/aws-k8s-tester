ARG CUDA_MAJOR_VERSION=13
ARG CUDA_MINOR_VERSION=0

# Use the NVIDIA CUDA runtime as a parent image
FROM nvidia/cuda:$CUDA_MAJOR_VERSION.$CUDA_MINOR_VERSION.0-devel-amzn2023

# Redeclare build arguments
ARG CUDA_MAJOR_VERSION
ARG CUDA_MINOR_VERSION

# Set default values for MASTER_ADDR, MASTER_PORT, and NUM_GPUS_PER_NODE
ENV MASTER_ADDR=127.0.0.1
ENV MASTER_PORT=12355

RUN dnf upgrade -y --releasever=latest \
 && dnf install -y \
        gcc \
        gcc-c++ \
        make \
        ca-certificates \
        cmake \
        git \
        jq \
        wget \
        tar \
        gzip \
        which \
        unzip \
        vim-enhanced \
        pkgconf-pkg-config \
        gdb \
        bzip2-devel \
        zlib-devel \
        openssl \
        openssl-devel \
        sqlite-devel \
        gdbm-devel \
        glibc-devel \
        xz-devel \
        ncurses-devel \
        tk-devel \
        libffi-devel \
        libcap-devel \
        libuuid-devel \
 && dnf clean all

# Install Python
ARG PYTHON=python3.10
ARG PYTHON_VERSION=3.10.12

RUN curl -sL https://www.python.org/ftp/python/$PYTHON_VERSION/Python-$PYTHON_VERSION.tgz | tar xvz -C /tmp \
 && cd /tmp/Python-$PYTHON_VERSION \
 && ./configure --enable-shared --prefix=/usr/local \
 && make -j $(nproc) \
 && make install \
 && echo /usr/local/lib > /etc/ld.so.conf.d/usr-local-lib.conf \
 && ldconfig \
 && cd && rm -rf /tmp/Python-$PYTHON_VERSION

RUN ln -s /usr/local/bin/pip3 /usr/bin/pip \
 && ln -s /usr/local/bin/$PYTHON /usr/local/bin/python \
 && pip --no-cache-dir install --upgrade pip setuptools

# Install Pytorch from Source
ARG PYTORCH_BRANCH=v2.9.1
ARG PYTORCH_BUILD_ENV="MAX_JOBS=8 BUILD_TEST=0"

ENV CUDA_HOME=/usr/local/cuda
ENV LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/usr/local/cuda/lib64:/usr/local/cuda/extras/CUPTI/lib64
ENV PATH=$PATH:$CUDA_HOME/bin
ENV TORCH_CUDA_ARCH_LIST="7.5;8.0;8.6;8.7;8.9;9.0;10.0;12.0"

RUN pip install typing-extensions sympy pyyaml "cmake>=3.27,<4"
RUN git clone https://github.com/pytorch/pytorch.git /tmp/pytorch \
        --recursive \
        --branch $PYTORCH_BRANCH \
 && cd /tmp/pytorch \
 && eval "$PYTORCH_BUILD_ENV python3 setup.py install" \
 && cd && rm -rf /tmp/pytorch

RUN dnf install -y \
    sudo git gcc gcc-c++ make vim-enhanced kmod openssh-clients openssh-server \
    wget autoconf libtool gdb automake cmake pkgconf-pkg-config hwloc-devel

RUN ldconfig

# SSH configuration
# AL2023 OpenSSH follows the host kernel's FIPS mode and then aborts on the
# curve25519 key exchange it offers by default, so drop it for FIPS hosts.
RUN ssh-keygen -A && \
    mkdir -p /var/run/sshd && \
    sed -i 's/[ #]\(.*StrictHostKeyChecking \).*/ \1no/g' /etc/ssh/ssh_config && \
    echo "    UserKnownHostsFile /dev/null" >> /etc/ssh/ssh_config && \
    sed -i 's/#\(StrictModes \).*/\1no/g' /etc/ssh/sshd_config && \
    echo "KexAlgorithms -curve25519-sha256,curve25519-sha256@libssh.org" > /etc/ssh/ssh_config.d/40-fips-kex.conf && \
    echo "KexAlgorithms -curve25519-sha256,curve25519-sha256@libssh.org" > /etc/ssh/sshd_config.d/40-fips-kex.conf

# Set environment variables for OpenMPI, CUDA, EFA, and NCCL
ENV LD_LIBRARY_PATH /opt/amazon/openmpi/lib64:/opt/amazon/openmpi/lib:/opt/amazon/efa/lib64:/opt/aws-ofi-nccl/install/lib:/usr/local/cuda/lib64:/usr/local/cuda/extras/CUPTI/lib64:/usr/local/lib/:/usr/lib64:$LD_LIBRARY_PATH
ENV PATH /usr/local/cuda/bin:/opt/amazon/openmpi/bin:/opt/amazon/efa/bin:/usr/sbin:/usr/bin:/usr/local/bin:$PATH

# Install EFA
ARG EFA_INSTALLER_VERSION=latest
RUN curl -sL https://efa-installer.amazonaws.com/aws-efa-installer-$EFA_INSTALLER_VERSION.tar.gz | tar xvz -C /tmp \
 && cd /tmp/aws-efa-installer \
 && ./efa_installer.sh -y -g -d --skip-kmod --skip-limit-conf --no-verify \
 && cd && rm -rf /tmp/aws-efa-installer

# Install NCCL
ARG LIBNCCL_VERSION=2.28.7-1
RUN git clone https://github.com/NVIDIA/nccl.git --branch v$LIBNCCL_VERSION /tmp/nccl \
 && cd /tmp/nccl \
 && make -j $(nproc) \
 && make install \
 && cd && rm -rf /tmp/nccl

# Install AWS-OFI-NCCL plugin
ARG AWS_OFI_NCCL_VERSION=1.17.2
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

RUN dnf clean all && rm -rf /var/cache/dnf

# Set the working directory in the container
WORKDIR /app

# Copy the training script and install requirements
COPY train.py /app/
COPY requirements.txt /app/
RUN pip install --no-cache-dir -r requirements.txt
