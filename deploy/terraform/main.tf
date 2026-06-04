# Один узел хостинга zerovibe на Timeweb Cloud.
# Создаёт: SSH-ключ + VM Ubuntu с cloud-init (Docker готов к старту).
# Доставку кода и запуск compose делает deploy/vm/deploy.sh после apply.

# Подбор ОС по имени/версии.
data "twc_os" "ubuntu" {
  name    = var.os_name
  version = var.os_version
}

# Подбор пресета конфигурации по ценовому диапазону в нужной локации.
data "twc_configurator" "conf" {
  location = var.location
}

data "twc_presets" "preset" {
  location = var.location

  price_filter {
    from = var.preset_price_from
    to   = var.preset_price_to
  }
}

# SSH-ключ для доступа к VM.
resource "twc_ssh_key" "deployer" {
  name = "${var.server_name}-key"
  body = var.ssh_public_key
}

# Виртуальная машина.
resource "twc_server" "node" {
  name              = var.server_name
  os_id             = data.twc_os.ubuntu.id
  preset_id         = data.twc_presets.preset.id
  availability_zone = var.location

  ssh_keys_ids = [twc_ssh_key.deployer.id]

  # cloud-init ставит Docker и готовит /opt/zerovibe.
  cloud_init = file("${path.module}/../vm/cloud-init.yaml")
}
