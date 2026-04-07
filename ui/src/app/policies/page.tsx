import { fetchMappingRules } from "@/lib/api";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";

// Dynamic Server Component rendering
export default async function PoliciesView() {
  // Graceful degradation during backend downtime
  let rules = [];
  try {
    rules = await fetchMappingRules();
  } catch (error) {
    console.error(error);
  }

  return (
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground">Policy Engine</h1>
        <p className="text-muted mt-2">Map Zero-Trust organizational borders enforcing specific propagation constraints.</p>
      </header>
      
      <Card>
        <div className="flex items-center justify-between mb-6">
          <CardTitle>Active Logic Rules</CardTitle>
          <button className="bg-primary hover:bg-primaryHover text-white px-4 py-2 rounded-md font-medium text-sm transition-all shadow-sm hover:shadow-md">
            + New Logic Rule
          </button>
        </div>
        
        {rules.length === 0 ? (
          <div className="text-center py-10 border border-dashed border-border rounded-lg">
             <p className="text-muted">No rules established. Systems are mutually exclusive.</p>
          </div>
        ) : (
          <div className="space-y-4">
             {rules.map((rule: any) => (
                <div key={rule.id} className="p-4 border border-border rounded-lg bg-surfaceHover flex items-center space-x-3 text-sm">
                  <span className="font-mono text-muted mr-2 font-semibold">IF</span>
                  <Badge variant="outline" className="border-primary text-primary">{rule.source_project}</Badge>
                  <Badge variant="secondary">{rule.source_role}</Badge>
                  
                  <span className="font-mono text-muted mx-2 font-semibold">THEN ADD</span>
                  
                  <Badge variant="outline" className="border-emerald-500 text-emerald-600 dark:text-emerald-400">{rule.target_project}</Badge>
                  <Badge variant="secondary">{rule.target_role}</Badge>
                </div>
             ))}
          </div>
        )}
      </Card>
    </div>
  );
}
