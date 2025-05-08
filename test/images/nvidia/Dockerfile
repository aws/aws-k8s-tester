ARG UBUNTU_MAJOR_VERSION=22

ARG CUDA_MAJOR_VERSION=12
ARG CUDA_MINOR_VERSION=8

# Start with the NVIDIA CUDA base image
FROM nvidia/cuda:$CUDA_MAJOR_VERSION.$CUDA_MINOR_VERSION.0-devel-ubuntu$UBUNTU_MAJOR_VERSION.04

ARG UBUNTU_MAJOR_VERSION

ARG CUDA_MAJOR_VERSION
ARG CUDA_MINOR_VERSION

ENV DEBIAN_FRONTEND=noninteractive

# Install necessary dependencies
RUN apt update -y \
 && apt remove -y --allow-change-held-packages \
      libmlx5-1 \
      ibverbs-utils \
      libibverbs-dev \
      libibverbs1 \
      libnccl2 \
      libnccl-dev \
 && rm -rf /opt/hpcx \
 && rm -rf /usr/local/mpi \
 && rm -rf /usr/local/ucx \
 && rm -f /etc/ld.so.conf.d/hpcx.conf \
 && apt install -y \
      git \
      gcc \
      openssh-client \
      openssh-server \
      build-essential \
      curl \
      autoconf \
      libtool \
      automake \
      cmake \
      apt-utils \
      libhwloc-dev \
      freeglut3-dev \
      libglu1-mesa-dev \
      datacenter-gpu-manager

RUN ldconfig

RUN mkdir -p /var/run/sshd \
 && sed -i 's/[ #]\(.*StrictHostKeyChecking \).*/ \1no/g' /etc/ssh/ssh_config \
 && echo "    UserKnownHostsFile /dev/null" >> /etc/ssh/ssh_config \
 && sed -i 's/#\(StrictModes \).*/\1no/g' /etc/ssh/sshd_config

# Build CUDA Samples
RUN git clone https://github.com/NVIDIA/cuda-samples.git /tmp/cuda-samples \
      --branch v$CUDA_MAJOR_VERSION.$CUDA_MINOR_VERSION \
 && cd /tmp/cuda-samples/Samples/0_Introduction/vectorAdd && cmake . && make -j$(nproc) && cp vectorAdd /usr/bin \
 && cd /tmp/cuda-samples/Samples/1_Utilities/deviceQuery && cmake . && make -j$(nproc) && cp deviceQuery /usr/bin \
 && cd /tmp/cuda-samples/Samples/1_Utilities/bandwidthTest && cmake . && make -j$(nproc) && cp bandwidthTest /usr/bin \
 && cd && rm -rf /tmp/cuda-samples

ENV LD_LIBRARY_PATH /opt/amazon/openmpi/lib:/opt/amazon/efa/lib:/opt/aws-ofi-nccl/install/lib:/usr/local/cuda/lib:/usr/local/lib/:/usr/lib64:/usr/lib/x86_64-linux-gnu/:/usr/lib/aarch64-linux-gnu/:$LD_LIBRARY_PATH
ENV PATH /usr/local/cuda/bin:/opt/amazon/openmpi/bin:/opt/amazon/efa/bin:/usr/sbin:/usr/bin:/usr/local/bin:$PATH

# Install EFA
ARG EFA_INSTALLER_VERSION=latest
RUN curl -sL https://efa-installer.amazonaws.com/aws-efa-installer-$EFA_INSTALLER_VERSION.tar.gz | tar xvz -C /tmp \
 && cd /tmp/aws-efa-installer \
 && ./efa_installer.sh --yes --enable-gdr --skip-kmod --skip-limit-conf --no-verify --mpi openmpi4 \
 && cd && rm -rf /tmp/aws-efa-installer

# Install NCCL
ARG LIBNCCL_VERSION=2.26.2-1
RUN apt update \
 && apt install -y \
      libnccl2=$LIBNCCL_VERSION+cuda$CUDA_MAJOR_VERSION.$CUDA_MINOR_VERSION \
      libnccl-dev=$LIBNCCL_VERSION+cuda$CUDA_MAJOR_VERSION.$CUDA_MINOR_VERSION \
 && rm -rf /var/lib/apt/lists/*

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

# Install NCCL Tests
ARG NCCL_TESTS_VERSION=2.15.0
RUN curl -sL https://github.com/NVIDIA/nccl-tests/archive/refs/tags/v$NCCL_TESTS_VERSION.tar.gz | tar xvz -C /tmp \
 && cd /tmp/nccl-tests-$NCCL_TESTS_VERSION \
 && make \
      MPI=1 \
      MPI_HOME=/opt/amazon/openmpi/ \
      CUDA_HOME=/usr/local/cuda \
 && mkdir -p /opt/nccl-tests \
 && mv build /opt/nccl-tests/build \
 && cd && rm -rf /tmp/nccl-tests-$NCCL_TESTS_VERSION

# Set a default command for debugging or modify as per requirements
ENV NCCL_PROTO simple
# see https://linux.die.net/man/8/ld.so for usage. replaces LD_PRELOAD env.
RUN echo "/usr/lib/$(uname -m)-linux-gnu/libnccl.so" >> /etc/ld.so.preload

RUN rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY test/images/nvidia/gpu_unit_tests ./gpu_unit_tests
RUN chmod +x ./gpu_unit_tests/unit_test
