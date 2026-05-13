import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { TrendingUp, Clock, ChevronRight } from 'lucide-react';

interface Category {
  id: number;
  name: string;
}

interface Market {
  id: number;
  question: string;
  category: Category;
  expiry: string;
  is_resolved: boolean;
  b_parameter: number;
}

export function Home() {
  const [markets, setMarkets] = useState<Market[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch('http://localhost:8000/markets')
      .then(res => res.json())
      .then(data => {
        setMarkets(data);
        setLoading(false);
      })
      .catch(err => {
        console.error("Failed to fetch markets", err);
        setLoading(false);
      });
  }, []);

  if (loading) {
    return (
      <div className="flex justify-center items-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-bold tracking-tight text-white mb-2">Live Markets</h1>
          <p className="text-slate-400">Trade on real-time events with Dual CLOB + AMM liquidity.</p>
        </div>
        <div className="bg-slate-800/50 p-3 rounded-2xl border border-slate-700/50 flex items-center gap-4">
          <div className="flex items-center gap-2 text-sm">
            <div className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></div>
            <span className="text-slate-300">Engine Online</span>
          </div>
        </div>
      </div>

      <div className="grid gap-4">
        {markets.length === 0 ? (
          <div className="text-center py-12 bg-slate-800/20 rounded-2xl border border-slate-700/50 border-dashed">
            <p className="text-slate-400">No active markets. Please create one via the admin panel.</p>
          </div>
        ) : (
          markets.map((market) => (
            <Link 
              key={market.id} 
              to={`/market/${market.id}`}
              className="group block bg-slate-800/40 hover:bg-slate-800/80 border border-slate-700/50 hover:border-blue-500/30 rounded-2xl p-6 transition-all duration-300 hover:shadow-lg hover:shadow-blue-500/5 relative overflow-hidden"
            >
              <div className="absolute inset-0 bg-gradient-to-r from-blue-500/0 via-blue-500/0 to-blue-500/5 opacity-0 group-hover:opacity-100 transition-opacity"></div>
              
              <div className="flex items-start justify-between">
                <div className="space-y-3">
                  <div className="flex items-center gap-3">
                    <span className="px-3 py-1 rounded-full text-xs font-medium bg-blue-500/10 text-blue-400 border border-blue-500/20">
                      {market.category ? market.category.name : "Uncategorized"}
                    </span>
                    <span className="flex items-center gap-1 text-xs text-slate-500">
                      <Clock className="w-3.5 h-3.5" />
                      {new Date(market.expiry).toLocaleDateString()}
                    </span>
                  </div>
                  <h3 className="text-xl font-semibold text-slate-100 group-hover:text-blue-400 transition-colors">
                    {market.question}
                  </h3>
                </div>
                
                <div className="flex items-center justify-center w-10 h-10 rounded-full bg-slate-800 border border-slate-700 group-hover:border-blue-500/30 group-hover:bg-blue-500/10 transition-colors">
                  <ChevronRight className="w-5 h-5 text-slate-400 group-hover:text-blue-400 transition-colors" />
                </div>
              </div>
            </Link>
          ))
        )}
      </div>
    </div>
  );
}
