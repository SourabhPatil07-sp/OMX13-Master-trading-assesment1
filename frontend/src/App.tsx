import { Routes, Route, Link } from 'react-router-dom';
import { Home } from './pages/Home';
import { MarketView } from './pages/MarketView';
import { Activity } from 'lucide-react';

function App() {
  return (
    <div className="min-h-screen bg-slate-900 text-slate-100 font-sans">
      <nav className="border-b border-slate-800 bg-slate-900/50 backdrop-blur-xl sticky top-0 z-50">
        <div className="max-w-5xl mx-auto px-4 h-16 flex items-center justify-between">
          <Link to="/" className="flex items-center gap-2 text-xl font-bold text-blue-500 tracking-tight hover:text-blue-400 transition-colors">
            <Activity className="w-6 h-6" />
            OmniMarket X
          </Link>
          <div className="flex items-center gap-4">
            <div className="bg-slate-800 px-4 py-1.5 rounded-full text-sm font-medium text-slate-300 border border-slate-700/50">
              User ID: 1
            </div>
            <div className="bg-emerald-500/10 text-emerald-400 px-4 py-1.5 rounded-full text-sm font-semibold border border-emerald-500/20">
              Balance: $1,000.00
            </div>
          </div>
        </div>
      </nav>

      <main className="max-w-5xl mx-auto px-4 py-8">
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/market/:id" element={<MarketView />} />
        </Routes>
      </main>
    </div>
  );
}

export default App;
