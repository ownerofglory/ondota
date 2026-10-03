{{- define "ondota.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "ondota.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := include "ondota.name" . }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "ondota.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "ondota.labels" -}}
helm.sh/chart: {{ include "ondota.chart" . }}
{{ include "ondota.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "ondota.selectorLabels" -}}
app.kubernetes.io/name: {{ include "ondota.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "ondota.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "ondota.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "ondota.secretName" -}}
{{- default (include "ondota.fullname" .) .Values.existingSecret }}
{{- end }}

{{- define "ondota.image" -}}
{{- printf "%s:%s" (required "image.repository is required" .Values.image.repository) (default .Chart.AppVersion .Values.image.tag) }}
{{- end }}
