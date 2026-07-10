import axios from 'axios'

axios.defaults.headers.post['Content-Type'] = 'application/x-www-form-urlencoded; charset=UTF-8'
axios.defaults.headers.common['X-Requested-With'] = 'XMLHttpRequest'

axios.defaults.baseURL = "./"
const pendingRequests = new Map<string, ReturnType<typeof axios.CancelToken.source>>()

const requestKey = (config: any): string => {
    if (String(config.method || '').toLowerCase() !== 'get') return ''
    const params = config.params && typeof config.params === 'object'
        ? Object.keys(config.params).sort().map((key) => [key, config.params[key]])
        : []
    return `get:${config.url}:${JSON.stringify(params)}`
}

const api = axios.create()

api.interceptors.request.use(
    (config) => {
        if (config.data instanceof FormData) {
            config.headers['Content-Type'] = 'multipart/form-data'
        }
        const key = requestKey(config)
        if (!key) return config
        
        // Check if there is already a pending request with the same key
        if (pendingRequests.has(key)) {
            pendingRequests.get(key)?.cancel('Duplicate request cancelled')
        }
        
        // Create a new cancel token for the request
        const cancelSource = axios.CancelToken.source()
        config.cancelToken = cancelSource.token
        
        // Store the cancel token in the pending requests map
        pendingRequests.set(key, cancelSource)
        
        return config
    },
    (error) => Promise.reject(error),
)

api.interceptors.response.use(
    (response) => {
        // Remove the request from the pending requests map
        const key = requestKey(response.config)
        if (key) pendingRequests.delete(key)
        return response
    },
    (error) => {
        if (axios.isCancel(error)) {
            // Handle duplicate request cancellation here if needed
            console.warn(error.message)
        } else {
            // Remove the request from the pending requests map on error
            const key = error.config ? requestKey(error.config) : ''
            if (key) pendingRequests.delete(key)
        }
        return Promise.reject(error)
    }
)

export default api
