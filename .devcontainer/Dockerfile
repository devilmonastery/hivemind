FROM golang:1.24-bookworm

# ============================================================================
# LAYER 1: Base system packages (rarely changes)
# ============================================================================
# Install Node.js (for Tailwind CSS build)
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs

# Install system dependencies
RUN apt-get update && apt-get install -y \
    git \
    curl \
    wget \
    postgresql-client \
    protobuf-compiler \
    sudo \
    zsh \
    locales \
    && rm -rf /var/lib/apt/lists/*

# Configure locales
RUN sed -i '/en_US.UTF-8/s/^# //g' /etc/locale.gen && \
    locale-gen

ENV LANG=en_US.UTF-8 \
    LANGUAGE=en_US:en \
    LC_ALL=en_US.UTF-8 \
    TZ=Europe/Berlin

# ============================================================================
# LAYER 2: User setup (rarely changes)
# ============================================================================
# Create non-root user
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

RUN groupadd --gid $USER_GID $USERNAME \
    && useradd --uid $USER_UID --gid $USER_GID -m -s /bin/zsh $USERNAME \
    && echo $USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$USERNAME \
    && chmod 0440 /etc/sudoers.d/$USERNAME

# Add vscode user to docker group for Docker-in-Docker access
# The docker group is created by the docker-in-docker feature
RUN groupadd -f docker && usermod -aG docker $USERNAME

# ============================================================================
# LAYER 3: Go environment setup (rarely changes)
# ============================================================================
# Set up Go environment for vscode user
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$PATH

# Create /go directory structure and set ownership to vscode user
# /go/pkg - for module cache and compiled packages
# /go/bin - for installed Go binaries
RUN mkdir -p /go/pkg /go/bin && chown -R $USER_UID:$USER_GID /go

# Create Go build cache directory with proper ownership
RUN mkdir -p /home/$USERNAME/.cache/go-build && \
    chown -R $USER_UID:$USER_GID /home/$USERNAME/.cache

# ============================================================================
# LAYER 4: Go tools (change this layer when updating tool versions)
# ============================================================================
# Switch to vscode user to install Go tools in their space
USER $USERNAME

RUN go install github.com/bufbuild/buf/cmd/buf@latest && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest && \
    go install golang.org/x/tools/gopls@latest

# ============================================================================
# LAYER 5: Final setup (most frequently modified workspace settings)
# ============================================================================
# Install Oh My Zsh for better shell experience
RUN sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended

WORKDIR /workspace
