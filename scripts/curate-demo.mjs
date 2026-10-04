// Publication of an explicitly reviewed editorial catalog. No automatic count-filling.
import fs from 'node:fs/promises';
const catalogPath=process.env.DEMO_CATALOG_PATH || 'docs/editorial-demo-catalog.json';
const catalog=JSON.parse(await fs.readFile(catalogPath,'utf8'));
const keys=['current','future_outcome','objective','trusted_source','sensible_cutoff','interesting'];
for(const entry of catalog.markets) {
 if(!keys.every(k=>entry.review?.[k]===true) || entry.review.rationale?.trim().length<30 || entry.description?.trim().length<30 || !entry.resolution_rule || !entry.resolution_source || !entry.resolution_time)throw new Error(`Incomplete editorial review: ${entry.title}`);
 if(/who wins on |Curated from the linked live official source/i.test(entry.title+' '+entry.description))throw new Error(`Fixture filler rejected: ${entry.title}`);
}
if(!process.argv.includes('--publish')) {
 console.log(JSON.stringify({reviewed:catalog.markets.length,categories:[...new Set(catalog.markets.map(m=>m.category))],publication:'Not requested'}));
} else {
 const api=process.env.DEMO_API_URL,token=process.env.DEMO_ADMIN_TOKEN;
 if(!api?.startsWith('https://')||!token)throw new Error('Configure the operator API and normal administrator session in environment variables');
 const request=async(path,method,body)=>{const response=await fetch(api+path,{method,headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json'},body:body?JSON.stringify(body):undefined,signal:AbortSignal.timeout(25000)});const data=await response.json();if(!response.ok)throw new Error(`${response.status}: ${data.error}`);return data;};
 let created=0,failed=0;
 for(const entry of catalog.markets){
  if(Date.parse(entry.lock_time)<=Date.now()) {console.error(`Cutoff passed: ${entry.title}`);failed++;continue;}
  try {
   const {review,featured,...market}=entry;
   const draft=await request('/markets','POST',market);
   const approved=await request(`/markets/${draft.id}/approve`,'POST',review);
   if(featured)await request(`/admin/markets/${draft.id}/feature`,'PUT',{featured:true});
   if(approved.market.resolution_status==='Live')created++;
   console.log(`Published ${draft.id}: ${market.title}`);
  }catch(error){failed++;console.error(`${entry.title}: ${error.message}`);}
 }
 const feed=await request('/live-feed?limit=100','GET');
 console.log(JSON.stringify({created,failed,verified_playable_count:feed.playable_count}));
 if(failed)process.exitCode=1;
}
