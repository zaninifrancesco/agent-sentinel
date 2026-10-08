import { useEffect, useState } from "react";
import type { SentinelConfig } from "./types";

const FALLBACK: SentinelConfig = { maxCostUsd: 0, maxTokens: 0, policy: "unknown", approvals: false, approvalTimeoutSec: 120 };

/** Loads what the running Sentinel was configured with (budget, approvals). */
export function useConfig(): SentinelConfig {
  const [config, setConfig] = useState<SentinelConfig>(FALLBACK);
  useEffect(() => {
    let cancelled = false;
    fetch("/api/config")
      .then((r) => (r.ok ? (r.json() as Promise<SentinelConfig>) : FALLBACK))
      .then((c) => {
        if (!cancelled) setConfig(c);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, []);
  return config;
}
