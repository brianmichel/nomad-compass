export class ApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly payload?: unknown,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

export interface HttpOptions extends RequestInit {
  json?: unknown;
}

export async function httpRequest<T = unknown>(input: RequestInfo | URL, options: HttpOptions = {}): Promise<T> {
  const { json, headers, ...init } = options;
  const finalHeaders = new Headers(headers ?? {});

  if (json !== undefined) {
    if (!finalHeaders.has('Content-Type')) {
      finalHeaders.set('Content-Type', 'application/json');
    }
    init.body = JSON.stringify(json);
  }

  const response = await fetch(input, { ...init, headers: finalHeaders });
  const data = await parseJson(response);

  if (!response.ok) {
    const message = extractErrorMessage(response, data);
    throw new ApiError(message, response.status, data);
  }

  return data as T;
}

async function parseJson(response: Response) {
  if (response.status === 204 || response.headers.get('content-length') === '0') {
    return undefined;
  }

  const contentType = response.headers.get('content-type');
  if (!contentType || !contentType.includes('application/json')) {
    return undefined;
  }

  try {
    return await response.json();
  } catch {
    return undefined;
  }
}

function extractErrorMessage(response: Response, data: unknown) {
  if (data && typeof data === 'object' && 'error' in data) {
    const error = (data as { error?: unknown }).error;
    if (typeof error === 'string' && error.trim()) {
      return error;
    }
  }

  if (response.statusText) {
    return response.statusText;
  }

  return 'Request failed';
}
