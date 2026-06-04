output "server_ip" {
  description = "Публичный IPv4 VM. Сюда направь A-record домена app_domain."
  value       = twc_server.node.main_ipv4
}

output "server_id" {
  description = "ID VM в Timeweb."
  value       = twc_server.node.id
}

output "deploy_hint" {
  description = "Следующий шаг после apply."
  value       = "1) A-record ${var.app_domain} → ${twc_server.node.main_ipv4}; 2) bash ../vm/deploy.sh ${twc_server.node.main_ipv4} ${var.app_domain}"
}
