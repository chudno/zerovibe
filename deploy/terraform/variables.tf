variable "server_name" {
  type        = string
  description = "Имя VM в панели Timeweb."
  default     = "zerovibe-node-1"
}

variable "location" {
  type        = string
  description = "Локация Timeweb (ru-1 = СПб). Для 152-ФЗ — РФ-локация."
  default     = "ru-1"
}

variable "os_name" {
  type        = string
  description = "Имя ОС."
  default     = "ubuntu"
}

variable "os_version" {
  type        = string
  description = "Версия ОС."
  default     = "22.04"
}

variable "preset_price_from" {
  type        = number
  description = "Нижняя граница цены пресета (руб/мес) для подбора конфигурации."
  default     = 200
}

variable "preset_price_to" {
  type        = number
  description = "Верхняя граница цены пресета (руб/мес)."
  default     = 600
}

variable "ssh_public_key" {
  type        = string
  description = "Публичный SSH-ключ для доступа к VM (содержимое ~/.ssh/id_ed25519.pub)."
}

variable "app_domain" {
  type        = string
  description = "Домен/поддомен приложения (A-record должен указывать на IP VM). Напр. app.zerovibe.ru."
}
