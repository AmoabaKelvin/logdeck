export interface ContainerInfo {
  id: string
  names: string[]
  image: string
  image_id: string
  command: string
  created: number
  state: string
  status: string
  labels?: Record<string, string>
}

export interface ContainersQueryParams {
  search?: string
  state?: string
  sortCreated?: "asc" | "desc"
  groupBy?: "none" | "compose"
}
