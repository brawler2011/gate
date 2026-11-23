{{- define "gate.tag" -}}
{{- required "imageTag is required (release-<commit>)" .Values.imageTag -}}
{{- end -}}

{{- define "gate.backendEnv" -}}
{{- $secret := .Values.secretName -}}
{{- range $key := list "POSTGRES_DSN" "S3_ACCESS_KEY" "S3_SECRET_KEY" "RESEND_API_KEY" }}
- name: {{ $key }}
  valueFrom: {secretKeyRef: {name: {{ $secret }}, key: {{ $key }}}}
{{- end }}
{{- end -}}

{{- define "gate.podSecurity" -}}
{{- with .Values.appArmorProfile -}}
appArmorProfile: {{- toYaml . | nindent 2 }}
{{ end -}}
seccompProfile: {type: RuntimeDefault}
{{- end -}}

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
