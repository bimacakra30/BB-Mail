#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_ENV="${ROOT_DIR}/backend/.env"
FRONTEND_ENV="${ROOT_DIR}/frontend/.env"
DEPLOY_DIR="${ROOT_DIR}/deploy"
MYSQL_CONTAINER="baramail-mysql"
BACKEND_CONTAINER="baramail-backend"

load_env() {
  if [[ ! -f "${BACKEND_ENV}" ]]; then
    echo "Error: File ${BACKEND_ENV} tidak ditemukan." >&2
    exit 1
  fi
  set -a
  # Strip CR characters in case .env was edited on Windows.
  source <(tr -d '\r' < "${BACKEND_ENV}")
  set +a
}

cmd_backup() {
  load_env

  if ! docker ps --format '{{.Names}}' | grep -qx "${MYSQL_CONTAINER}"; then
    echo "Error: Container ${MYSQL_CONTAINER} tidak sedang berjalan." >&2
    exit 1
  fi

  local timestamp
  timestamp="$(date -u +%Y%m%d-%H%M%S)"
  local out_file="${1:-${ROOT_DIR}/baramail-backup-${timestamp}.tar.gz}"
  local tmp_dir
  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "${tmp_dir}"' EXIT

  local db_name="${DB_NAME:-baramail}"
  local root_pass="${DB_ROOT_PASSWORD:-${DB_PASSWORD:-change_root_password}}"

  echo "[1/3] Mengambil dump database '${db_name}' dari container ${MYSQL_CONTAINER}..."
  docker exec "${MYSQL_CONTAINER}" mysqldump \
    -u root \
    -p"${root_pass}" \
    --single-transaction \
    --routines \
    --triggers \
    --no-tablespaces \
    "${db_name}" > "${tmp_dir}/db_dump.sql"

  echo "[2/3] Menyalin file konfigurasi .env..."
  cp "${BACKEND_ENV}" "${tmp_dir}/backend.env"
  if [[ -f "${FRONTEND_ENV}" ]]; then
    cp "${FRONTEND_ENV}" "${tmp_dir}/frontend.env"
  fi

  echo "[3/3] Membungkus arsip backup..."
  tar -czf "${out_file}" -C "${tmp_dir}" .

  echo "Backup selesai: ${out_file} ($(du -h "${out_file}" | cut -f1))"
}

cmd_restore() {
  local archive_file="${1:-}"
  if [[ -z "${archive_file}" || ! -f "${archive_file}" ]]; then
    echo "Usage: ./migrate.sh restore <path-ke-file-backup.tar.gz>" >&2
    exit 1
  fi

  local tmp_dir
  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "${tmp_dir}"' EXIT

  echo "[1/5] Mengekstrak arsip backup ${archive_file}..."
  tar -xzf "${archive_file}" -C "${tmp_dir}"

  if [[ ! -f "${tmp_dir}/backend.env" || ! -f "${tmp_dir}/db_dump.sql" ]]; then
    echo "Error: Arsip backup tidak valid (backend.env atau db_dump.sql tidak ditemukan)." >&2
    exit 1
  fi

  echo "[2/5] Memulihkan file konfigurasi .env..."
  cp "${tmp_dir}/backend.env" "${BACKEND_ENV}"
  if [[ -f "${tmp_dir}/frontend.env" ]]; then
    cp "${tmp_dir}/frontend.env" "${FRONTEND_ENV}"
  fi
  ln -sf ../backend/.env "${DEPLOY_DIR}/.env"

  load_env

  echo "[3/5] Menjalankan Docker Compose..."
  (cd "${DEPLOY_DIR}" && docker compose up -d --build)

  echo "[4/5] Menunggu container ${MYSQL_CONTAINER} siap (healthy)..."
  local retries=30
  while [[ ${retries} -gt 0 ]]; do
    local status
    status="$(docker inspect -f '{{.State.Health.Status}}' "${MYSQL_CONTAINER}" 2>/dev/null || echo "starting")"
    if [[ "${status}" == "healthy" ]]; then
      break
    fi
    sleep 2
    retries=$((retries - 1))
  done

  if [[ ${retries} -eq 0 ]]; then
    echo "Error: Container ${MYSQL_CONTAINER} belum healthy setelah 60 detik." >&2
    exit 1
  fi

  local db_name="${DB_NAME:-baramail}"
  local root_pass="${DB_ROOT_PASSWORD:-${DB_PASSWORD:-change_root_password}}"

  echo "[5/5] Merestore database '${db_name}' dan me-restart backend..."
  docker exec -i "${MYSQL_CONTAINER}" mysql \
    -u root \
    -p"${root_pass}" \
    "${db_name}" < "${tmp_dir}/db_dump.sql"

  docker restart "${BACKEND_CONTAINER}" >/dev/null

  echo "Restore berhasil! BaraMail aktif di port WEB_PORT=${WEB_PORT:-8085}."
}

case "${1:-}" in
  backup)
    cmd_backup "${2:-}"
    ;;
  restore)
    cmd_restore "${2:-}"
    ;;
  *)
    echo "Usage:"
    echo "  ./migrate.sh backup [nama-file-output.tar.gz]"
    echo "  ./migrate.sh restore <file-backup.tar.gz>"
    exit 1
    ;;
esac

