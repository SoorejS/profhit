// Operator curation from live public provider data. No fixtures, fabricated
// publication times, odds, player counts or outcomes are inserted.
import fs from 'node:fs/promises';
const now = new Date();
const catalogPath = process.env.DEMO_CATALOG_PATH || 'audit-artifacts/genuine-demo-catalog.json';
const cities = [
 ['Washington DC',38.9072,-77.0369], ['New York',40.7128,-74.0060],
 ['Boston',42.3601,-71.0589], ['Chicago',41.8781,-87.6298],
 ['Seattle',47.6062,-122.3321], ['Miami',25.7617,-80.1918],
 ['Dallas',32.7767,-96.7970], ['Denver',39.7392,-104.9903]
];
async function getJSON(url, options={}) {
 const response=await fetch(url,{...options,headers:{'User-Agent':'PROPHIT demo curation','Content-Type':'application/json',...options.headers},signal:AbortSignal.timeout(25000)});
 if(!response.ok)throw new Error(`Provider/request status ${response.status}`);
 return response.json();
}
function base(category,title,sourceURL,sourceName,published,cutoff,resultTime,rule,resultSource,options=['Yes','No']) {
 return {category,title,description:'Curated from the linked live official source. No result has been declared.',difficulty:'Easy',entry_coins:10,options:JSON.stringify(options),news_url:sourceURL,news_source_name:sourceName,news_published_at:published,source_kind:'official_event',lock_time:cutoff.toISOString(),resolution_time:resultTime.toISOString(),resolution_rule:rule,resolution_source:resultSource,resolution_status:'Draft',visibility:'Public'};
}
async function collect() {
 const markets=[];
 const date=new Date(now.getTime()+24*3600000).toISOString().slice(0,10).replaceAll('-','');
 try {
  const data=await getJSON(`https://site.api.espn.com/apis/site/v2/sports/football/nfl/scoreboard?dates=${date}`);
  for(const event of data.events||[]) {
   const start=new Date(event.date), competitors=event.competitions?.[0]?.competitors;
   const source=event.links?.find(link=>link.href?.startsWith('https://www.espn.com/'))?.href;
   if(event.status?.type?.name!=='STATUS_SCHEDULED'||start<=new Date(now.getTime()+3600000)||competitors?.length!==2||!source)continue;
   const options=[...competitors.map(item=>item.team.displayName),'Draw'];
   const resultTime=new Date(start.getTime()+5*3600000);
   markets.push(base('Sports',`${event.name}: who wins on ${event.date.slice(0,10)}?`,source,'ESPN official schedule',null,start,resultTime,`Use the final official ESPN result for game ${event.id}. Choose the winning team; choose Draw if the final scores are equal. Postponed or abandoned games remain awaiting resolution until an official final result is available.`,source,options));
  }
 }catch(error){console.error(`Sports curation unavailable: ${error.message}`);}
 for(const [city,lat,lon] of cities) {
  try {
   const point=await getJSON(`https://api.weather.gov/points/${lat},${lon}`);
   const forecastURL=point.properties.forecast;
   const stationListURL=point.properties.observationStations;
   if(!forecastURL?.startsWith('https://api.weather.gov/')||!stationListURL?.startsWith('https://api.weather.gov/'))throw new Error('Official weather links unavailable');
   const forecast=await getJSON(forecastURL), stations=await getJSON(stationListURL);
   const nearby=(stations.features||[]).filter(item=>item.properties?.stationIdentifier&&item.geometry?.coordinates?.length===2).sort((a,b)=>Math.hypot(a.geometry.coordinates[0]-lon,a.geometry.coordinates[1]-lat)-Math.hypot(b.geometry.coordinates[0]-lon,b.geometry.coordinates[1]-lat));
   const station=nearby[0];if(!station)throw new Error('Observation station unavailable');
   const stationID=station.properties.stationIdentifier;
   const resultSource=`https://api.weather.gov/stations/${encodeURIComponent(stationID)}/observations`;
   const published=forecast.properties.updateTime;
   if(!published||!Number.isFinite(Date.parse(published))||new Date(published)>now)throw new Error('Actual forecast update time unavailable');
   for(const period of (forecast.properties.periods||[]).filter(p=>p.isDaytime).slice(0,6)) {
    if(period.temperatureUnit!=='F'||!Number.isFinite(period.temperature))continue;
    const observedFrom=new Date(Date.parse(period.startTime)+6*3600000), observedTo=new Date(observedFrom.getTime()+3600000), cutoff=new Date(observedFrom.getTime()-3600000);
    if(cutoff<=new Date(now.getTime()+3600000))continue;
    const threshold=period.temperature;
    const title=`${city}, ${observedFrom.toISOString().slice(0,16)} UTC: temperature ≥ ${threshold}°F?`;
    const rule=`At NWS station ${stationID}, use the first valid temperature observation timestamped from ${observedFrom.toISOString()} inclusive to ${observedTo.toISOString()} exclusive. Convert Celsius to Fahrenheit using C × 9/5 + 32. Yes if temperature is at least ${threshold}°F; otherwise No. Missing data remains awaiting resolution; do not substitute a forecast or another station.`;
    const market=base('Weather',title,forecastURL,'National Weather Service',published,cutoff,observedTo,rule,resultSource);
    market.description=`Official NWS forecast updated ${published} for ${city}: ${period.name}, ${threshold}°F. This question concerns an actual future observation at ${station.properties.name} (${stationID}), not the forecast value as an outcome.`;
    markets.push(market);
   }
  }catch(error){console.error(`${city} curation unavailable: ${error.message}`);}
 }
 try {
  const coins={bitcoin:'Bitcoin',ethereum:'Ethereum',solana:'Solana',cardano:'Cardano',dogecoin:'Dogecoin'};
  const url=`https://api.coingecko.com/api/v3/simple/price?ids=${Object.keys(coins).join(',')}&vs_currencies=usd&include_last_updated_at=true`;
  const data=await getJSON(url);
  for(const [id,name] of Object.entries(coins)) {
   const record=data[id], observed=new Date(Number(record?.last_updated_at)*1000);
   if(!Number.isFinite(record?.usd)||record.usd<=0||!Number.isFinite(observed.getTime())||observed>now||now-observed>20*60000)continue;
   const cutoff=new Date(now.getTime()+24*3600000), resultTime=new Date(cutoff.getTime()+5*60000);
   const source=`https://api.coingecko.com/api/v3/simple/price?ids=${id}&vs_currencies=usd&include_last_updated_at=true`;
   markets.push(base('Financial Markets',`${name}: USD price direction at ${cutoff.toISOString().slice(0,16)} UTC?`,source,'CoinGecko live USD price',observed.toISOString(),cutoff,resultTime,`Baseline: ${record.usd} USD, CoinGecko last_updated_at ${observed.toISOString()}. Use the first CoinGecko USD quote with last_updated_at from ${cutoff.toISOString()} inclusive to ${resultTime.toISOString()} exclusive. Up if greater than baseline, Down if less, Unchanged if exactly equal. If no valid quote is recorded in that window, retain awaiting resolution; never fabricate a price.`,source,['Up','Down','Unchanged']));
  }
 }catch(error){console.error(`Financial curation unavailable: ${error.message}`);}
 await fs.writeFile(catalogPath,JSON.stringify({collected_at:now.toISOString(),markets},null,2));
 console.log(`Collected ${markets.length} genuine future opportunities; no fixtures. Catalog: ${catalogPath}`);
}
async function publish() {
 const api=process.env.DEMO_API_URL, token=process.env.DEMO_ADMIN_TOKEN;
 if(!api?.startsWith('https://')||!token)throw new Error('Set DEMO_API_URL and DEMO_ADMIN_TOKEN in the operator environment');
 const catalog=JSON.parse(await fs.readFile(catalogPath,'utf8'));
 let created=0,duplicates=0,failed=0;
 for(const market of catalog.markets) {
  if(new Date(market.lock_time)<=new Date())continue;
  try {
   const response=await fetch(`${api}/markets`,{method:'POST',headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json'},body:JSON.stringify(market),signal:AbortSignal.timeout(25000)});
   if(response.status===409){duplicates++;continue;}
   const result=await response.json();if(!response.ok)throw new Error(result.error||`Create failed ${response.status}`);
   const approved=await getJSON(`${api}/markets/${result.id}/approve`,{method:'POST',headers:{Authorization:`Bearer ${token}`}});
   if(approved.market?.resolution_status==='Live')created++;
  }catch(error){failed++;console.error(`Market ${market.title}: ${error.message}`);}
 }
 const feed=await getJSON(`${api}/live-feed`);
 console.log(JSON.stringify({created,duplicates,failed,verified_playable_count:feed.playable_count}));
}
if(process.argv.includes('--publish'))await publish();else await collect();
