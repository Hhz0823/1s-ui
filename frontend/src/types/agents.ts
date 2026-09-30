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
  load1?: number
  tcp_conns?: number
  udp_conns?: number
}

// One point of stored history from api/agents/:id/metrics.
export type AgentMetricPoint = {
  time: number
  cpu: number
  mem: number
  swap: number
  disk: number
  load1: number
  processes: number
  tcp: number
  udp: number
  net_sent_rate: number
  net_recv_rate: number
  net_sent: number
  net_recv: number
  ping_ms: number
  ping_loss: number
}

export type AgentLatencyPoint = { time: number, ms: number, ok: boolean }

export type TrafficLimitType = 'sum' | 'max' | 'min' | 'up' | 'down'

// Display metadata the admin sets on the monitor page.
export type AgentNodeMeta = {
  group: string
  tags: string
  region: string
  remark: string
  price: number
  currency: string
  billing_cycle: number
  expire_at: number
  sort_weight: number
  traffic_limit: number
  traffic_limit_type: TrafficLimitType
  traffic_reset_day: number
}

export type AgentCommand = {
  id: string
  type: string
  ok: boolean
  output?: string
  error?: string
  elapsed_ms?: number
}

export type AgentNode = Partial<AgentNodeMeta> & {
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
  traffic?: { sent: number, recv: number, period_start: number, next_reset: number }
  latency_history?: AgentLatencyPoint[]
  report: {
    hostname?: string
    os?: string
    arch?: string
    uptime?: number
    cpu_percent?: number
    cpu_cores?: number
    cpu_model?: string
    platform?: string
    kernel?: string
    virtualization?: string
    tcp_conns?: number
    udp_conns?: number
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
