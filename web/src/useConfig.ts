import { useCallback, useEffect, useState } from "react";
import type { SentinelConfig } from "./types";

const FALLBACK: SentinelConfig = {
  maxCostUsd: 0,
  maxTokens: 0,
  policy: "unknown",
  approvals: false,
  budgetEditable: false,
  approvalTimeoutSec: 120,
};

export interface ConfigState {
  config: SentinelConfig;
  /** Use a config the server just returned (e.g. after a budget change). */
  apply: (c: SentinelConfig) => void;
  /** Ask the server again: the limits move when a human approves going over. */
  reload: () => void;
}

/** Loads what the running Sentinel was configured with (budget, approvals). */
export function useConfig(): ConfigState {
  const [config, setConfig] = useState<SentinelConfig>(FALLBACK);

  const reload = useCallback(() => {
    fetch("/api/config")
      .then((r) => (r.ok ? (r.json() as Promise<SentinelConfig>) : FALLBACK))
      .then(setConfig)
      .catch(() => {});
  }, []);

  useEffect(reload, [reload]);
  return { config, apply: setConfig, reload };
}
