"use client";

import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend
} from "recharts";
import { UsagePoint } from "../lib/api";

export default function UsageChart({ data }: { data: UsagePoint[] }) {
  // Format data for chart
  const chartData = data.map(point => ({
    ...point,
    timestamp: new Date(point.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    cost: parseFloat(point.totalCost.replace('$', ''))
  })).reverse(); // Show oldest to newest

  return (
    <div className="h-[400px] w-full bg-white p-6 rounded-xl border border-slate-200 mb-8">
      <h3 className="text-lg font-semibold mb-6">Token Usage (24h)</h3>
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart
          data={chartData}
          margin={{
            top: 10,
            right: 30,
            left: 0,
            bottom: 0,
          }}
        >
          <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#e2e8f0" />
          <XAxis 
            dataKey="timestamp" 
            stroke="#64748b" 
            fontSize={12} 
            tickLine={false}
            axisLine={false}
          />
          <YAxis 
            stroke="#64748b" 
            fontSize={12} 
            tickLine={false}
            axisLine={false}
            tickFormatter={(value) => `${value}`}
          />
          <Tooltip 
            contentStyle={{ 
              backgroundColor: '#fff', 
              borderRadius: '8px', 
              border: '1px solid #e2e8f0',
              boxShadow: '0 4px 6px -1px rgb(0 0 0 / 0.1)' 
            }}
          />
          <Legend />
          <Area 
            type="monotone" 
            dataKey="totalTokens" 
            name="Total Tokens"
            stackId="1" 
            stroke="#2563eb" 
            fill="#3b82f6" 
            fillOpacity={0.1} 
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}

