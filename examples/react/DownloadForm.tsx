// types.ts
export type JobStatus = "pending" | "processing" | "done" | "error";

export interface DownloadResponse {
  job_id: string;
}

export interface StatusResponse {
  id: string;
  status: JobStatus;
  progress: number;
  title?: string;
  download_url?: string;
  error?: string;
  created_at: string;
  completed_at?: string;
}

export interface ErrorResponse {
  error: string;
  code: string;
}

// DownloadForm.tsx
import { useState, useCallback, useEffect, useRef } from "react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api";
const POLL_INTERVAL = 2000;

// Hook personalizado para o download
export function useDownload() {
  const [jobId, setJobId] = useState<string | null>(null);
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const pollRef = useRef<NodeJS.Timeout | null>(null);

  // Limpar polling ao desmontar
  useEffect(() => {
    return () => {
      if (pollRef.current) {
        clearInterval(pollRef.current);
      }
    };
  }, []);

  // Função para iniciar download
  const startDownload = useCallback(
    async (url: string, turnstileToken: string) => {
      setIsLoading(true);
      setError(null);
      setStatus(null);

      try {
        const response = await fetch(`${API_URL}/download`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "X-Turnstile-Token": turnstileToken,
          },
          body: JSON.stringify({ url }),
        });

        const data = await response.json();

        if (!response.ok) {
          throw new Error(data.error || "Erro ao iniciar download");
        }

        setJobId(data.job_id);

        // Iniciar polling
        const poll = async () => {
          try {
            const statusRes = await fetch(`${API_URL}/status/${data.job_id}`);
            const statusData: StatusResponse = await statusRes.json();
            setStatus(statusData);

            if (statusData.status === "done" || statusData.status === "error") {
              if (pollRef.current) {
                clearInterval(pollRef.current);
                pollRef.current = null;
              }
              setIsLoading(false);
            }
          } catch (err) {
            console.error("Erro ao verificar status:", err);
          }
        };

        poll(); // Poll imediato
        pollRef.current = setInterval(poll, POLL_INTERVAL);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Erro desconhecido");
        setIsLoading(false);
      }
    },
    []
  );

  // Reset
  const reset = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
    setJobId(null);
    setStatus(null);
    setError(null);
    setIsLoading(false);
  }, []);

  return {
    jobId,
    status,
    isLoading,
    error,
    startDownload,
    reset,
  };
}

// Componente de progresso
interface ProgressBarProps {
  progress: number;
}

export function ProgressBar({ progress }: ProgressBarProps) {
  return (
    <div className="w-full h-2 bg-gray-200 rounded-full overflow-hidden">
      <div
        className="h-full bg-gradient-to-r from-indigo-500 to-purple-500 transition-all duration-300"
        style={{ width: `${progress}%` }}
      />
    </div>
  );
}

// Status Badge
interface StatusBadgeProps {
  status: JobStatus;
}

export function StatusBadge({ status }: StatusBadgeProps) {
  const colors = {
    pending: "bg-yellow-100 text-yellow-800",
    processing: "bg-blue-100 text-blue-800",
    done: "bg-green-100 text-green-800",
    error: "bg-red-100 text-red-800",
  };

  const labels = {
    pending: "Na fila",
    processing: "Baixando",
    done: "Concluído",
    error: "Erro",
  };

  return (
    <span
      className={`px-3 py-1 rounded-full text-sm font-medium ${colors[status]}`}
    >
      {labels[status]}
    </span>
  );
}

// Formulário principal
interface DownloadFormProps {
  turnstileToken?: string;
  onTurnstileExpired?: () => void;
}

export default function DownloadForm({
  turnstileToken,
  onTurnstileExpired,
}: DownloadFormProps) {
  const [url, setUrl] = useState("");
  const { status, isLoading, error, startDownload, reset } = useDownload();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!turnstileToken) {
      alert("Complete a verificação de segurança");
      return;
    }

    await startDownload(url, turnstileToken);
  };

  const handleReset = () => {
    reset();
    setUrl("");
    onTurnstileExpired?.();
  };

  return (
    <div className="w-full max-w-md mx-auto p-6">
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label
            htmlFor="url"
            className="block text-sm font-medium text-gray-700 mb-1"
          >
            URL do Vídeo
          </label>
          <input
            type="url"
            id="url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="https://www.youtube.com/watch?v=..."
            className="w-full px-4 py-3 border border-gray-300 rounded-lg focus:ring-2 focus:ring-indigo-500 focus:border-transparent"
            required
            disabled={isLoading}
          />
        </div>

        {/* Turnstile widget seria renderizado aqui */}

        <button
          type="submit"
          disabled={isLoading || !turnstileToken}
          className="w-full py-3 px-4 bg-gradient-to-r from-indigo-500 to-purple-500 text-white font-semibold rounded-lg hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed transition-opacity"
        >
          {isLoading ? "Processando..." : "Baixar Vídeo"}
        </button>
      </form>

      {/* Status */}
      {error && (
        <div className="mt-4 p-4 bg-red-50 border border-red-200 rounded-lg">
          <p className="text-red-700">{error}</p>
          <button onClick={handleReset} className="mt-2 text-red-600 underline">
            Tentar novamente
          </button>
        </div>
      )}

      {status && (
        <div className="mt-4 p-4 bg-gray-50 border border-gray-200 rounded-lg space-y-3">
          <div className="flex items-center justify-between">
            <span className="font-medium">
              {status.title || "Processando..."}
            </span>
            <StatusBadge status={status.status} />
          </div>

          {status.status !== "error" && (
            <ProgressBar progress={status.progress} />
          )}

          {status.status === "done" && status.download_url && (
            <a
              href={status.download_url}
              className="inline-flex items-center px-4 py-2 bg-green-500 text-white rounded-lg hover:bg-green-600 transition-colors"
              download
            >
              ⬇️ Download
            </a>
          )}

          {status.status === "error" && (
            <p className="text-red-600">{status.error}</p>
          )}
        </div>
      )}
    </div>
  );
}
