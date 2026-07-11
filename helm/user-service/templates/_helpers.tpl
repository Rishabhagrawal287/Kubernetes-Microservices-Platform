{{/*
Shared label used everywhere this chart needs to select or tag its own
pods. Kept deliberately simple (a single "app" label) rather than the full
app.kubernetes.io/* convention, since this chart is only ever installed
once per environment — no need for Helm's multi-release-name complexity here.
*/}}
{{- define "user-service.labels" -}}
app: user-service
{{- end -}}
