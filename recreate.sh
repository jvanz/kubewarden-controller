k3d cluster delete
k3d cluster create --verbose --wait -v /dev/mapper:/dev/mapper

#cert manager
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.5.3/cert-manager.yaml
kubectl wait --for=condition=Available deployment --timeout=2m -n cert-manager --all
#open telemetry
kubectl apply -f https://github.com/open-telemetry/opentelemetry-operator/releases/latest/download/opentelemetry-operator.yaml
kubectl wait --for=condition=Available deployment --timeout=2m -n opentelemetry-operator-system --all
#prometheus
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts

kubectl create namespace kubewarden

#docker run --rm \
#  --name jaeger \
#  -p14250:14250 \
#  -p16686:16686 \
#  jaegertracing/all-in-one:1.27.0
#
#JAEGER_IP=`docker container inspect -f '{{ .NetworkSettings.IPAddress }}' jaeger`
#echo "Jaeger IP: ${JAEGER_IP}"
#
#docker run --rm \
#  -p 4317:4317 \
#  -p 8889:8889 \
#  -v `pwd`/otel-collector-minimal-config.yaml:/etc/otel/config.yaml:ro \
#  otel/opentelemetry-collector:0.36.0 \
#    --log-level debug \
#    --config /etc/otel/config.yaml
#
#docker run -d --rm \
#  --add-host=host.docker.internal:host-gateway \
#  -p 9090:9090 \
#  -v $(pwd)/prometheus.yml:/etc/prometheus/prometheus.yml \
#  prom/prometheus:v2.30.3
