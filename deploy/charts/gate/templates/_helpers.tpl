{{- define "gate.tag" -}}
{{- required "imageTag is required (release-<commit>)" .Values.imageTag -}}
{{- end -}}

{{/* Secrets and connection strings of the backend, also used by the migration Job.
   deploy/secrets.sh builds the URLs with escaped passwords. */}}
{{- define "gate.backendEnv" -}}
{{- $secret := .Values.secretName -}}
{{- range $key := list "POSTGRES_DSN" "REDIS_URL" "S3_ACCESS_KEY" "S3_SECRET_KEY" "RESEND_API_KEY" }}
- name: {{ $key }}
  valueFrom: {secretKeyRef: {name: {{ $secret }}, key: {{ $key }}}}
{{- end }}
- name: NATS_URL
  value: nats://nats:4222
{{- end -}}

{{/* Pod security shared by every pod; appArmorProfile is per environment. */}}
{{- define "gate.podSecurity" -}}
{{- with .Values.appArmorProfile -}}
appArmorProfile: {{- toYaml . | nindent 2 }}
{{ end -}}
seccompProfile: {type: RuntimeDefault}
{{- end -}}

{{/* Probes of the backend container in API and worker pods; both serve /health. */}}
{{- define "gate.backendProbes" -}}
readinessProbe:
  httpGet: {path: /health, port: http}
  periodSeconds: 5
  failureThreshold: 2
livenessProbe:
  httpGet: {path: /health, port: http}
  periodSeconds: 10
  failureThreshold: 6
{{- end -}}
