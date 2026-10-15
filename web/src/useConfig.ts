import { useCallback, useEffect, useState } from "react";
import type { SentinelConfig } from "./types";

const FALLBACK: SentinelConfig = {
  maxCostUsd: 0,
  maxTokens: 0,
  policy: "unknown",
  approvals: false,
  budgetEditable: false,
  approvalTimeoutSec: 0,
};

export interface ConfigState {
  config: SentinelConfig;

}

/** Reads capture mode; legacy fields remain available for older session reports. */
export function useConfig(): ConfigState {
  const [config, setConfig] = useState<SentinelConfig>(FALLBACK);

  const reload = useCallback(() => {
    fetch("/api/config")
      .then((r) => (r.ok ? (r.json() as Promise<SentinelConfig>) : FALLBACK))
      .then(setConfig)
      .catch(() => {});
  }, []);

  useEffect(reload, [reload]);
  return { config };
}
