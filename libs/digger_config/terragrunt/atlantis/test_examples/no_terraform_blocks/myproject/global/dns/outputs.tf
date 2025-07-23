output "public_dns_zones_ids" {
  description = "The zone IDs"
  value = {
    for zone in cloudflare_zone.external :
    zone.zone => zone.id
  }
}
