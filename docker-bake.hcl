APP_NAME = "ani4s"
FRAMEWORK = "golang-gin"
APP_VERSION = "v1.0.0"
DB_PASS = "VerySecurePassword"
DB_NAME = "ani4s-db"
DB_USER = "admin"
DB_VERSION = "16-bullseye"
REDIS_VERSION = "8.0.2-alpine"
group "default" {
  targets = ["app", "db", "redis"]
}

target "app" {
  context = "."
  dockerfile = "Dockerfile"
  tags = ["${APP_NAME}-${FRAMEWORK}:${APP_VERSION}"]
  platforms = ["linux/amd64"]
  args = {
    APP_NAME = APP_NAME
    APP_VERSION = APP_VERSION
  }
}

target "db" {
  context = "."
  dockerfile = "Dockerfile.psql"
  tags = [
    "${APP_NAME}-postgres:${DB_VERSION}"
  ]
  platforms = ["linux/amd64"]
  args = {
    POSTGRES_DB = DB_NAME
    POSTGRES_USER = DB_USER
    POSTGRES_PASSWORD = DB_PASS
  }
}
target "redis" {
  context = "."
  dockerfile = "Dockerfile.redis"
  tags = ["${APP_NAME}-redis:${REDIS_VERSION}"]
  platforms = ["linux/amd64"]
}

