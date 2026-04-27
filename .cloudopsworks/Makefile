##
# (c) 2021-2025
#     Cloud Ops Works LLC - https://cloudops.works/
#     Find us on:
#       GitHub: https://github.com/cloudopsworks
#       WebSite: https://cloudops.works
#     Distributed Under Apache v2.0 License
#
OS := $(shell uname)
PWD := $(shell pwd)
CURR := $(shell basename $(PWD))
DATE :=	$(shell date +%Y%m%d-%H%M%S.%s)
VER_NUM := $(shell cat _VERSION)
VER_MAJOR := $(shell echo $(VER_NUM) | cut -f1 -d.)
VER_MINOR := $(shell echo $(VER_NUM) | cut -f2 -d.)
VER_PATCH := $(shell echo $(VER_NUM) | cut -f3 -d.)

.PHONY: tag
.PHONY: tag_local
.PHONY: version

co_master:
	git checkout master

tag_local: co_master
	git tag -f $(VER_MAJOR).$(VER_MINOR)
	git tag -f $(VER_MAJOR)

tag: tag_local
	git push origin -f $(VER_MAJOR).$(VER_MINOR)
	git push origin -f $(VER_MAJOR)
	git switch -
