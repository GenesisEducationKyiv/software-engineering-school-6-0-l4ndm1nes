#!/bin/bash
set -euo pipefail

exec > /var/log/user-data.log 2>&1

echo "=== Installing Docker ==="
dnf update -y
dnf install -y docker git
systemctl start docker
systemctl enable docker
usermod -aG docker ec2-user

echo "=== Installing Docker Compose ==="
DOCKER_COMPOSE_VERSION=$(curl -s https://api.github.com/repos/docker/compose/releases/latest | grep '"tag_name"' | cut -d'"' -f4)
curl -L "https://github.com/docker/compose/releases/download/$${DOCKER_COMPOSE_VERSION}/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose

echo "=== Installing Nginx ==="
dnf install -y nginx
systemctl enable nginx

echo "=== Cloning application ==="
mkdir -p /opt/app
cd /opt/app
git clone https://github.com/GenesisBlock3301/GitHub-Release-Notification-API.git app || true
cd app

echo "=== Creating .env ==="
cat > .env << 'ENVEOF'
SERVER_PORT=8080
BASE_URL=http://${app_domain}
SERVER_READ_TIMEOUT=15s
SERVER_WRITE_TIMEOUT=15s
SERVER_IDLE_TIMEOUT=60s
SERVER_SHUTDOWN_TIMEOUT=10s

LOG_LEVEL=info

DB_HOST=${db_host}
DB_PORT=${db_port}
DB_USER=${db_user}
DB_PASSWORD=${db_password}
DB_NAME=${db_name}
DB_SSLMODE=require
DB_QUERY_TIMEOUT=5s
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=10
DB_CONN_MAX_LIFETIME=5m

REDIS_ADDR=${redis_addr}
REDIS_PASSWORD=
REDIS_DB=0
REDIS_DIAL_TIMEOUT=5s
REDIS_READ_TIMEOUT=3s
REDIS_WRITE_TIMEOUT=3s
REDIS_MAX_RETRIES=3

SMTP_HOST=email-smtp.${aws_region}.amazonaws.com
SMTP_PORT=587
SMTP_USER=
SMTP_PASSWORD=
SMTP_FROM=${smtp_from}
SMTP_TIMEOUT=10s

GITHUB_TOKEN=${github_token}
GITHUB_BASE_URL=https://api.github.com
GITHUB_TIMEOUT=15s
GITHUB_MAX_RETRIES=3
GITHUB_CACHE_TTL=10m

SCANNER_INTERVAL=5m
SCANNER_CYCLE_TIMEOUT=4m
SCANNER_REPO_TIMEOUT=30s

GRPC_PORT=50051

API_KEY=${api_key}
ENVEOF

echo "=== Creating docker-compose.prod.yml ==="
cat > docker-compose.prod.yml << 'COMPEOF'
services:
  app:
    build: .
    env_file: .env
    environment:
      GIN_MODE: release
    ports:
      - "127.0.0.1:8080:8080"
      - "0.0.0.0:50051:50051"
    restart: unless-stopped
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
COMPEOF

echo "=== Building and starting application ==="
docker-compose -f docker-compose.prod.yml up --build -d

echo "=== Configuring Nginx ==="
cat > /etc/nginx/conf.d/app.conf << 'NGINXEOF'
server {
    listen 80;
    server_name _;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 30s;
        proxy_send_timeout 30s;
    }
}
NGINXEOF

rm -f /etc/nginx/conf.d/default.conf
nginx -t && systemctl restart nginx

echo "=== Setup complete ==="
