
 ####################
 # Docker Functions #
 ####################

# Returns 1 if the container is running, 0 otherwise
# examples:
# - $(call container_is_running,foo)
define container_is_running
	$(shell docker ps | grep -w -c $(1))
endef

# Returns the port that is exposed in the host for a container
# examples:
# - $(call container_get_port,foo,8080)
# - $(call container_get_port,bar,53/udp)
define container_get_port
	$(shell docker port $(1) $(2) | cut -d":" -f2)
endef
