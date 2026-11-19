import { useQuery } from "@tanstack/react-query";
export const base = "/api/targets/local";
export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(base + path, init);
  if (!res.ok)
    throw new Error((await res.text()).trim() || `HTTP ${res.status}`);
  return res.json();
}
export function useData<T>(path: string, interval = 5000) {
  return useQuery({
    queryKey: ["local", path],
    queryFn: ({ signal }) => api<T>(path, { signal }),
    refetchInterval: interval,
    retry: false,
  });
}
export interface Container {
  ID: string;
  Name: string;
  Project: string;
  Service: string;
  Managed: boolean;
  Image: string;
  State: string;
  Status: string;
  Mounts:
    { Type: string; Source: string; Destination: string; RW: boolean }[] | null;
  Ports:
    | { PrivatePort: number; PublicPort: number; IP: string; Type: string }[]
    | null;
  Networks: Record<
    string,
    { EndpointID: string; IPAddress: string; GlobalIPv6Address: string }
  > | null;
}
export interface Inventory {
  Containers: Container[];
  Networks: {
    Name: string;
    ID: string;
    Driver: string;
    IPAM: { Config: { Subnet: string }[] | null };
  }[];
}
export interface Run {
  job_id: string;
  Status: string;
  Repository: string;
  Trigger: string;
  created_at: string;
  Deployments: { Stack: string; Context: string }[] | null;
}
export interface Deployments {
  Projects: { Name: string; Status: string }[] | null;
  Runs: Run[] | null;
  ProjectsError: string;
  RunsError: string;
}
export interface Source {
  ID: string;
  Name: string;
  URL: string;
  Reference: string;
  Target: string;
  Scope: string;
}
export interface Settings {
  write: boolean;
  control: boolean;
  poll: boolean;
  sources: Source[] | null;
}
export interface Job {
  Run: Run;
  Error: string;
}
export const labels: Record<string, string> = {
  accepted: "Angenommen",
  running: "Läuft",
  succeeded: "Erfolgreich",
  failed: "Fehlgeschlagen",
  skipped: "Übersprungen",
  unknown: "Lauf nicht mehr auffindbar",
  created: "Erstellt",
  exited: "Beendet",
  paused: "Pausiert",
  restarting: "Startet neu",
  dead: "Defekt",
  start: "Starten",
  stop: "Stoppen",
  restart: "Neu starten",
  recreate: "Neu erstellen",
  sync: "Abgleichen",
};
