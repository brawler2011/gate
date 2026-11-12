{{- define "gate.tag" -}}
{{- required "imageTag is required (release-<commit>)" .Values.imageTag -}}
{{- end -}}

{{/* Secrets and connection strings of the backend, also used by the migration Job. */}}
{{- define "gate.backendEnv" -}}
{{- $secret := .Values.secretName -}}
{{- range $key := list "POSTGRES_PASSWORD" "REDIS_PASSWORD" "S3_ACCESS_KEY" "S3_SECRET_KEY" "RESEND_API_KEY" }}
- name: {{ $key }}
  valueFrom: {secretKeyRef: {name: {{ $secret }}, key: {{ $key }}}}
{{- end }}
# Passwords are URL-safe by convention (deploy/README.md), so they are not escaped.
- name: POSTGRES_DSN
  value: postgres://postgres:$(POSTGRES_PASSWORD)@postgres:5432/{{ .Values.postgresDatabase }}?sslmode=disable
- name: REDIS_URL
  value: redis://:$(REDIS_PASSWORD)@redis:6379/0
- name: NATS_URL
  value: nats://nats:4222
{{- end -}}
