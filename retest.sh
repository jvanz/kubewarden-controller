#!/bin/bash

make install
export KUBEWARDEN_DEVELOPMENT_MODE=1 
export WEBHOOK_HOST_LISTEN=$(docker inspect k3d-k3s-default-server-0 | jq -r '.[] | .NetworkSettings.Networks."k3d-k3s-default".Gateway') 
make run
