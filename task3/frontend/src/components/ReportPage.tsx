import React, { useState, useEffect } from 'react';

export interface ReportData {
  user_id: string;
  full_name: string;
  email: string;
  registered_at?: string;
  period_start: string;
  period_end: string;
  usage_hours: number;
  session_count: number;
  avg_daily_use: number;
  updated_at: string;
}

export interface ReportResponse {
  report_url: string;
}

const ReportPage: React.FC = () => {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [authenticated, setAuthenticated] = useState<boolean | null>(null);
  const [reportUrl, setReportUrl] = useState<string | null>(null);

  const authUrl = process.env.REACT_APP_AUTH_URL || 'http://localhost:8001';

  useEffect(() => {
    fetch(`${authUrl}/session/check`, { credentials: 'include' })
      .then((res) => setAuthenticated(res.ok))
      .catch(() => setAuthenticated(false));
  }, [authUrl]);

  const handleLogin = () => {
    window.location.href = `${authUrl}/login`;
  };

  const handleLogout = () => {
    window.location.href = `${authUrl}/logout`;
  };

  const fetchReport = async () => {
    try {
      setLoading(true);
      setError(null);
      setReportUrl(null);

      const response = await fetch(`${authUrl}/api/reports`, {
        credentials: 'include',
      });

      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || `HTTP ${response.status}: ${response.statusText}`);
      }

      const data: ReportResponse = await response.json();
      setReportUrl(data.report_url);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred');
    } finally {
      setLoading(false);
    }
  };

  if (authenticated === null) {
    return <div className="flex items-center justify-center min-h-screen">Загрузка...</div>;
  }

  if (!authenticated) {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen bg-gray-100">
        <button
          onClick={handleLogin}
          className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
        >
          Login
        </button>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center justify-center min-h-screen bg-gray-100">
      <div className="p-8 bg-white rounded-lg shadow-md">
        <div className="flex justify-between items-center mb-6">
          <h1 className="text-2xl font-bold">Отчёты по использованию</h1>
        </div>

        <div className="flex justify-between items-center mb-6">
          <button
              onClick={handleLogout}
              className="text-sm text-gray-500 hover:text-gray-700"
            >
            Выйти
          </button>
        </div>

        <button
          onClick={fetchReport}
          disabled={loading}
          className={`px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 ${
            loading ? 'opacity-50 cursor-not-allowed' : ''
          }`}
        >
          {loading ? 'Загрузка…' : 'Получить отчёт'}
        </button>

        {error && (
          <div className="mt-4 p-4 bg-red-100 text-red-700 rounded">
            {error}
          </div>
        )}

        {reportUrl && (
          <div className="mt-6 p-4 bg-gray-50 rounded border text-left">
            <h2 className="text-lg font-semibold mb-3">Ваш отчёт</h2>
            <p className="mb-3 text-sm text-gray-600">Отчёт сформирован и доступен по ссылке. Ваша ссылка защищена и действует ограниченное время.</p>
            <a
              href={reportUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-block px-4 py-2 bg-green-600 text-white rounded hover:bg-green-700"
            >
              Открыть отчёт в новой вкладке
            </a>
            <iframe
              title="Отчёт"
              src={reportUrl}
              className="mt-4 w-full border rounded min-h-[400px]"
              sandbox="allow-same-origin"
            />
          </div>
        )}
      </div>
    </div>
  );
};

export default ReportPage;
