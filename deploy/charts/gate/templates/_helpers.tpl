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
appArmorProfile: {{- toYaml .Values.appArmorProfile | nindent 2 }}
seccompProfile: {type: RuntimeDefault}
{{- end -}}
