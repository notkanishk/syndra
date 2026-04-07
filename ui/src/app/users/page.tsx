"use client";

import { useState, useEffect, useCallback } from "react";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";

interface Bundle {
  id: string;
  name: string;
  description: string;
}

export default function UsersView() {
  const [userId, setUserId] = useState("");
  const [userBundles, setUserBundles] = useState<Bundle[]>([]);
  const [allBundles, setAllBundles] = useState<Bundle[]>([]);
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState("");

  const loadAllBundles = useCallback(async () => {
    try {
      const res = await fetch("/api/proxy/bundles");
      const data = await res.json();
      setAllBundles(Array.isArray(data) ? data : []);
    } catch (err) {
      console.error("Failed to load bundles", err);
    }
  }, []);

  const loadUserBundles = async (uid: string) => {
    if (!uid) return;
    setLoading(true);
    setMessage("");
    try {
      const res = await fetch(`/api/proxy/users/${uid}/bundles`);
      const data = await res.json();
      setUserBundles(Array.isArray(data) ? data : []);
    } catch (err) {
      setUserBundles([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadAllBundles();
  }, [loadAllBundles]);

  const handleAssign = async (bundleId: string) => {
    if (!userId) {
      setMessage("Please enter a User ID first");
      return;
    }
    try {
      const res = await fetch(`/api/proxy/users/${userId}/bundles`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ bundle_id: bundleId }),
      });
      if (res.ok) {
        setMessage("Bundle assigned successfully!");
        loadUserBundles(userId);
      } else {
        const body = await res.json();
        setMessage(`Error: ${body.message || "Failed to assign"}`);
      }
    } catch (err) {
      setMessage("Failed to assign bundle");
    }
  };

  return (
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground">Users & Access</h1>
        <p className="text-muted mt-2">Manage bundle assignments and trace permission lineage.</p>
      </header>

      <Card>
        <div className="mb-8">
          <label className="block text-sm font-semibold text-foreground mb-2">Search User by Zitadel ID</label>
          <div className="flex gap-3">
            <input
              type="text"
              placeholder="e.g. 1928374655"
              value={userId}
              onChange={(e) => setUserId(e.target.value)}
              className="flex-1 px-4 py-2 rounded-md border border-border bg-surface text-foreground focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary transition-all"
            />
            <button 
              onClick={() => loadUserBundles(userId)}
              className="bg-primary hover:bg-primaryHover text-white px-6 py-2 rounded-md font-medium transition-all shadow-sm"
            >
              Load Access
            </button>
          </div>
          {message && <p className="mt-2 text-xs text-primary font-medium">{message}</p>}
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
          {/* Assigned Bundles */}
          <div className="space-y-4">
            <h3 className="text-sm font-bold text-muted uppercase tracking-widest">Currently Assigned</h3>
            {loading ? (
              <p className="text-sm text-muted">Loading...</p>
            ) : userBundles.length === 0 ? (
              <div className="p-8 border border-dashed border-border rounded-lg text-center">
                <p className="text-sm text-muted italic">No bundles assigned yet.</p>
              </div>
            ) : (
              <div className="space-y-2">
                {userBundles.map(b => (
                  <div key={b.id} className="p-3 bg-surfaceHover border border-border rounded-md flex justify-between items-center">
                    <span className="font-medium text-sm">{b.name}</span>
                    <Badge variant="secondary">Assigned</Badge>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Available Bundles */}
          <div className="space-y-4">
            <h3 className="text-sm font-bold text-muted uppercase tracking-widest">Available Bundles</h3>
            <div className="space-y-2 max-h-[300px] overflow-y-auto pr-2">
              {allBundles.length === 0 ? (
                <p className="text-sm text-muted">No bundles found in system.</p>
              ) : (
                allBundles.map(b => {
                  const isAssigned = userBundles.some(ub => ub.id === b.id);
                  return (
                    <div key={b.id} className="p-3 border border-border rounded-md group hover:border-primary/50 transition-colors">
                      <div className="flex justify-between items-start">
                        <div>
                          <p className="text-sm font-medium">{b.name}</p>
                          <p className="text-[10px] text-muted">{b.description}</p>
                        </div>
                        <button 
                          onClick={() => handleAssign(b.id)}
                          disabled={isAssigned}
                          className={`text-[10px] uppercase font-bold py-1 px-2 rounded tracking-wider transition-all
                            ${isAssigned 
                              ? 'bg-muted/10 text-muted cursor-default' 
                              : 'bg-primary/10 text-primary hover:bg-primary hover:text-white'
                            }`}
                        >
                          {isAssigned ? 'Assigned' : '+ Assign'}
                        </button>
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          </div>
        </div>
      </Card>
      
      <Card className="bg-primary/5 border-primary/10 mt-10">
        <h4 className="text-sm font-bold text-primary mb-2">Permission Logic</h4>
        <p className="text-xs text-muted leading-relaxed">
          Assigning a bundle grants all its internal roles immediately. The Data Plane will automatically resolve these 
          plus any <strong>Mapping Rules</strong> triggered by these roles.
        </p>
      </Card>
    </div>
  );
}
