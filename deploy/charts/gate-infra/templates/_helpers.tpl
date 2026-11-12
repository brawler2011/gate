{{/* Pod security shared by every pod; appArmorProfile is per environment. */}}
{{- define "gate-infra.podSecurity" -}}
appArmorProfile: {type: {{ .Values.appArmorProfile }}}
seccompProfile: {type: RuntimeDefault}
{{- end -}}
