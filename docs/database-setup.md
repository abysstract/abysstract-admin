# 本地数据库准备

本文说明如何准备本地数据库。数据库要求与应用连接配置见[数据库配置](configuration.md#数据库配置)。已有数据库服务时，创建应用专用数据库后填写连接信息即可。

本文提供本地开发的准备方法。Docker 是其中一种选择。使用 Docker 命令前，先安装并启动 Docker，确认 `docker compose version` 可以执行。

## PostgreSQL

在仓库根目录执行：

```bash
repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"
docker compose up -d --wait
```

命令启动 PostgreSQL 容器，等待数据库就绪后退出。数据库继续在后台运行。首次启动会下载镜像。

仓库配置使用以下连接信息：

| 配置项 | 值 |
|---|---|
| 地址 | `localhost:5433` |
| 用户名 | `admin` |
| 密码 | `admin` |
| 自带后台数据库 | `abysstract_admin`，首次启动时自动创建 |

业务接入教程继续使用这里创建的数据库。

创建后，返回 [README 快速开始](../README.md#快速开始体验自带后台)或[模块教程](new-module-tutorial.md)，继续配置连接和执行迁移。

## MySQL 和 MariaDB

可以使用已有服务，也可以通过官方 Docker 镜像准备本地服务。下面两个方案任选一个。

MySQL 示例使用本机端口 `3307`：

```bash
docker run -d --name admin-mysql \
  -p 127.0.0.1:3307:3306 \
  -e MYSQL_ROOT_PASSWORD=local-root-password \
  -e MYSQL_DATABASE=abysstract_admin \
  -e MYSQL_USER=admin -e MYSQL_PASSWORD=admin \
  -v admin_mysql_data:/var/lib/mysql mysql:8.4
```

MariaDB 示例使用本机端口 `3308`：

```bash
docker run -d --name admin-mariadb \
  -p 127.0.0.1:3308:3306 \
  -e MARIADB_ROOT_PASSWORD=local-root-password \
  -e MARIADB_DATABASE=abysstract_admin \
  -e MARIADB_USER=admin -e MARIADB_PASSWORD=admin \
  -v admin_mariadb_data:/var/lib/mysql mariadb:11.4.13
```

这些命令创建容器后立即退出。查看所选容器的日志，等待数据库输出 `ready for connections`，再连接数据库：

```bash
# MySQL
docker logs admin-mysql
# MariaDB
docker logs admin-mariadb
```

容器中的数据库名为 `abysstract_admin`，用户名和密码均为 `admin`。按[数据库配置](configuration.md#连接示例)填写连接信息，地址使用所选示例的本机端口。业务接入教程沿用该数据库。

## SQLite 本地文件

SQLite 直接使用本机文件。应用首次连接时创建数据库文件。

创建存放数据库的目录：

```bash
mkdir -p "$HOME/abysstract-admin-data"
```

按[SQLite 配置](configuration.md#sqlite-本机文件)填写该目录下的绝对文件路径。业务接入教程沿用该文件。

## 停止与再次启动

使用仓库的 PostgreSQL 服务时，在仓库根目录执行：

```bash
docker compose stop
# 再次使用时启动数据库：
docker compose up -d --wait
```

使用上面的 MySQL 或 MariaDB 容器时，对所选容器执行 `docker stop admin-mysql` 或 `docker stop admin-mariadb`。再次使用时执行对应的 `docker start` 命令，并检查启动日志。

这些停止命令会保留数据库数据。SQLite 数据保存在配置指定的文件中，应用退出后仍然保留。
