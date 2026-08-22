export type AgentUsage = {
  used?: number
  total?: number
}

export type AgentMetricSample = {
  time: number
  cpu_percent: number
  mem_percent: number
  swap_percent?: number
  disk_percent?: number
  process_count?: number
  net_sent_rate?: number
  net_recv_rate?: number
}

export type AgentCommand = {
  id: string
  type: string
  ok: boolean
  output?: string
  error?: string
  elapsed_ms?: number
}

export type AgentNode = {
  id: number
  name: string
  created_at?: number
  last_seen: number
  remote_ip: string
  public_host?: string
  version: string
  online: boolean
  conn_mode?: string
  ws_connected?: boolean
  controllable?: boolean
  managed?: boolean
  latency?: {
    last_ms?: number | null
    average_ms?: number
    p95_ms?: number
    loss_pct?: number
    samples?: number
    updated_at?: number
  }
  commands?: AgentCommand[]
  report: {
    hostname?: string
    os?: string
    arch?: string
    uptime?: number
    cpu_percent?: number
    cpu_cores?: number
    memory?: AgentUsage
    swap?: AgentUsage
    disk?: AgentUsage
    network?: { sent?: number, recv?: number }
    net_rate?: { sent?: number, recv?: number }
    load?: { load1?: number, load5?: number, load15?: number }
    process_count?: number
    ipv4?: string[]
    ipv6?: string[]
    cores?: { singbox_running?: boolean, xray_running?: boolean, xray_version?: string }
    panel?: { installed?: boolean, version?: string, public_url?: string, control_available?: boolean, protocol_version?: number, capabilities?: string[] }
    conn_mode?: string
  }
  history?: AgentMetricSample[]
}
