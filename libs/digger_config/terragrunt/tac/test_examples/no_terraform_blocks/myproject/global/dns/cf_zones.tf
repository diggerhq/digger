resource "cloudflare_zone" "external" {
  for_each = var.public_dns_zones
  zone     = each.value
}
