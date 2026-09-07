resource "tsuga_custom_usage_tag" "custom_usage_tag" {
  tag_key = "service.name"
}

import {
  to = tsuga_custom_usage_tag.custom_usage_tag
  id = "usage-1"
}
