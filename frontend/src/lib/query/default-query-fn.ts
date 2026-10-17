import type { QueryFunction } from "@tanstack/react-query";

// Wails binding calls will be wired here as features land.
export const defaultQueryFn: QueryFunction = async () => {
  return null;
};
