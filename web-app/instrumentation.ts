import { SeverityNumber, type LogRecord } from "@opentelemetry/api-logs"
import { OTLPLogExporter } from "@opentelemetry/exporter-logs-otlp-http"
import { resourceFromAttributes } from "@opentelemetry/resources"
import { BatchLogRecordProcessor, LoggerProvider } from "@opentelemetry/sdk-logs"

const posthogKey = process.env.NEXT_PUBLIC_POSTHOG_KEY
const posthogHost = process.env.NEXT_PUBLIC_POSTHOG_HOST

const loggerProvider = posthogKey && posthogHost
  ? new LoggerProvider({
      resource: resourceFromAttributes({ "service.name": "pursuit-web-app" }),
      processors: [
        new BatchLogRecordProcessor({
          exporter: new OTLPLogExporter({
            url: `${posthogHost.replace(/\/$/, "")}/i/v1/logs`,
            headers: {
              Authorization: `Bearer ${posthogKey}`,
              "Content-Type": "application/json",
            },
            timeoutMillis: 2000,
          }),
        }),
      ],
    })
  : undefined

const logger = loggerProvider?.getLogger("pursuit.posthog")

if (!loggerProvider && process.env.NODE_ENV !== "production") {
  const missingVariable = posthogKey ? "NEXT_PUBLIC_POSTHOG_HOST" : "NEXT_PUBLIC_POSTHOG_KEY"
  console.warn(
    `${missingVariable} variable required by PostHog is missing or un-configured, this causes events to be silently missed. This error stops appearing once ${missingVariable} is configured`,
  )
}

export function register() {}

export function logPostHog(record: Pick<LogRecord, "body" | "severityNumber" | "attributes">) {
  logger?.emit(record)
}

export function flushPostHogLogs() {
  return loggerProvider?.forceFlush().catch(() => {})
}

export { SeverityNumber }
