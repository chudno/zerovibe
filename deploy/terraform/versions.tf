terraform {
  required_version = ">= 1.5"
  required_providers {
    twc = {
      source  = "timeweb-cloud/timeweb-cloud"
      version = "~> 1.6"
    }
  }
}

# Токен берётся из переменной окружения TWC_TOKEN (не храним в коде).
provider "twc" {}
