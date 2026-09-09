ARG CUDA_MAJOR_VERSION=12
ARG CUDA_MINOR_VERSION=8

# Start with the NVIDIA CUDA base image
FROM nvidia/cuda:$CUDA_MAJOR_VERSION.$CUDA_MINOR_VERSION.0-devel-ubuntu22.04

ARG CUDA_MAJOR_VERSION
ARG CUDA_MINOR_VERSION

ENV DEBIAN_FRONTEND=noninteractive

# Install necessary dependencies
RUN apt update -y \
 && apt upgrade -y \
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
      datacenter-gpu-manager-4-cuda12 \
      datacenter-gpu-manager-4-cuda13

RUN ldconfig

ENV LD_LIBRARY_PATH /opt/amazon/openmpi/lib:/opt/amazon/efa/lib:/opt/aws-ofi-nccl/install/lib:/usr/local/cuda/lib:/usr/local/lib/:/usr/lib64:/usr/lib/x86_64-linux-gnu/:/usr/lib/aarch64-linux-gnu/:$LD_LIBRARY_PATH
ENV PATH /usr/local/cuda/bin:/opt/amazon/openmpi/bin:/opt/amazon/efa/bin:/usr/sbin:/usr/bin:/usr/local/bin:$PATH

RUN mkdir -p /var/run/sshd \
 && sed -i 's/[ #]\(.*StrictHostKeyChecking \).*/ \1no/g' /etc/ssh/ssh_config \
 && echo "    UserKnownHostsFile /dev/null" >> /etc/ssh/ssh_config \
 && sed -i 's/#\(StrictModes \).*/\1no/g' /etc/ssh/sshd_config

# Build CUDA Samples.
# Each entry is "<relative-path-under-Samples>:<produced-binary-name>".
# When cuda-samples upstream reorganizes a sample, update its path here.
RUN git clone https://github.com/NVIDIA/cuda-samples.git /tmp/cuda-samples \
      --branch v$CUDA_MAJOR_VERSION.$CUDA_MINOR_VERSION \
 && for sample in \
      "0_Introduction/vectorAdd:vectorAdd" \
      "1_Utilities/deviceQuery:deviceQuery" \
      "6_Performance/UnifiedMemoryPerf:UnifiedMemoryPerf" \
      "3_CUDA_Features/immaTensorCoreGemm:immaTensorCoreGemm" \
      "2_Concepts_and_Techniques/reductionMultiBlockCG:reductionMultiBlockCG" \
      "2_Concepts_and_Techniques/shfl_scan:shfl_scan" \
      "0_Introduction/simpleAWBarrier:simpleAWBarrier" \
      "0_Introduction/simpleAtomicIntrinsics:simpleAtomicIntrinsics" \
      "0_Introduction/simpleVoteIntrinsics:simpleVoteIntrinsics" \
      "3_CUDA_Features/warpAggregatedAtomicsCG:warpAggregatedAtomicsCG" \
      "3_CUDA_Features/globalToShmemAsyncCopy:globalToShmemAsyncCopy"; do \
      dir="${sample%%:*}"; bin="${sample##*:}"; \
      cd "/tmp/cuda-samples/Samples/$dir" && cmake . && make -j$(nproc) && cp "$bin" /usr/bin || exit 1; \
    done \
 && cd && rm -rf /tmp/cuda-samples

# Install EFA
ARG EFA_INSTALLER_VERSION=latest
RUN curl -sL https://efa-installer.amazonaws.com/aws-efa-installer-$EFA_INSTALLER_VERSION.tar.gz | tar xvz -C /tmp \
 && cd /tmp/aws-efa-installer \
 && ./efa_installer.sh --yes --enable-gdr --skip-kmod --skip-limit-conf --no-verify --mpi openmpi4 \
 && cd && rm -rf /tmp/aws-efa-installer

# Build nvbandwidth
ARG NVBANDWIDTH_VERSION=v0.8
RUN apt install -y libboost-program-options-dev
RUN git clone https://github.com/NVIDIA/nvbandwidth.git --branch $NVBANDWIDTH_VERSION /tmp/nvbandwidth \
 && cd /tmp/nvbandwidth \
 && cmake -DMULTINODE=1 . && make && cp nvbandwidth /usr/bin \
 && cd && rm -rf /tmp/cuda-samples

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

# Install NCCL Tests
# TODO: automate pin with version bump
RUN git clone https://github.com/NVIDIA/nccl-tests /tmp/nccl-tests \
 && cd /tmp/nccl-tests \
 && make \
      MPI=1 \
      MPI_HOME=/opt/amazon/openmpi/ \
      CUDA_HOME=/usr/local/cuda \
      NCCL_HOME=/usr/local/lib \
 && mkdir -p /opt/nccl-tests \
 && mv build /opt/nccl-tests/build \
 && cd && rm -rf /tmp/nccl-tests

# Set a default command for debugging or modify as per requirements
ENV NCCL_PROTO simple
# see https://linux.die.net/man/8/ld.so for usage. replaces LD_PRELOAD env.
RUN echo "/usr/local/lib/libnccl.so" >> /etc/ld.so.preload

RUN rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY test/images/nvidia/gpu_unit_tests ./gpu_unit_tests
RUN chmod +x ./gpu_unit_tests/unit_test ./gpu_unit_tests/dcgm-diag.sh
