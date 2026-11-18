{{- define "gate-infra.podSecurity" -}}
{{- with .Values.appArmorProfile -}}
appArmorProfile: {{- toYaml . | nindent 2 }}
{{ end -}}
seccompProfile: {type: RuntimeDefault}
{{- end -}}
