from pathlib import Path
import os,sys,subprocess as sp,time,json,hashlib,ctypes as ct,math,statistics as ss,concurrent.futures as cf,shutil,platform,tempfile,re,threading,sqlite3,uuid
B=Path(__file__).resolve().parent
R=Path(tempfile.gettempdir())/("obit-audit-6926-"+hashlib.sha256(str(B).encode()).hexdigest()[:10]);R.mkdir(exist_ok=True)
K=ct.windll.kernel32;K.CreateMutexW.restype=ct.c_void_p
repo=R/"repo"; repo.mkdir(exist_ok=True)
base={k:v for k,v in os.environ.items() if not k.startswith(("OBIT_","GIT_","CLAUDE_","CODEX_"))};base.update(OBIT_TEST_NO_UPDATE="1",OBIT_TELEMETRY="0",DO_NOT_TRACK="1",OBIT_LANG="en",OBIT_TEST_REGISTRY_ROOT=r"Software\obituary-test-692603",GIT_CONFIG_NOSYSTEM="1",GIT_CONFIG_GLOBAL=str(R/"gitconfig"),GIT_CEILING_DIRECTORIES=str(R),GIT_TERMINAL_PROMPT="0",PYTHONUTF8="1")
for key,sub in dict(OBIT_USER_HOME="user",HOME="user",USERPROFILE="user",APPDATA="roaming",LOCALAPPDATA="local",CODEX_HOME="codex",CLAUDE_CONFIG_DIR="claude",OBIT_CONFIG_DIR="config",XDG_CONFIG_HOME="config").items():
 p=R/sub;p.mkdir(exist_ok=True);base[key]=str(p)
(R/"config"/"config.toml").write_text("[proposals]\nmode=\"tiered\"\n[explorer]\nenabled=false\n[update]\ncheck=false\n[agent_shells]\ncodex=\"bash\"\n",encoding="utf8")
def env(name):
 p=R/name/"home";p.mkdir(parents=True,exist_ok=True);return dict(base,OBIT_HOME=str(p))
def run(ex,args,env,payload=None,allow=False):
 data=None if payload is None else json.dumps(payload).encode(); t=time.perf_counter_ns();r=sp.run([str(B/(ex+".exe")),*args],input=data,stdout=sp.PIPE,stderr=sp.PIPE,cwd=repo,env=env,creationflags=sp.CREATE_NO_WINDOW,timeout=120); d=(time.perf_counter_ns()-t)/1e6
 if not allow and r.returncode:raise RuntimeError((args,r.returncode,r.stderr[-1500:],r.stdout[-500:]))
 return d,r.stdout,r.stderr
def show(name,v):
 a=sorted(v);print(name,len(a),round(ss.median(a),3),round(a[math.ceil(.95*len(a))-1],3),flush=True)
 with open(B/(sys.argv[1]+"-raw.jsonl"),"a",encoding="utf8") as f: print(json.dumps([name,v]),file=f)
def seed(e,n):run("probe",["seed",str(n),str(repo)],e)
def state(e):return json.loads((Path(e["OBIT_HOME"])/"store.json").read_bytes())
def mutex(e):
 import csv
 sid=next(csv.reader([sp.check_output(["whoami","/user","/fo","csv","/nh"],encoding="utf8").strip()]))[1]; nm=r"Local\Obituary-obitd-"+sid+"-"+hashlib.sha256(e["OBIT_HOME"].upper().encode()).hexdigest()[:32];h=K.CreateMutexW(None,False,nm);assert h;return h
def payload(agent,kind,sid,path=None):
 ev,sub=kind.split("/");p=dict(cwd=str(repo),source=sub,session_id=sid,hook_event_name=ev)
 if ev=="PreToolUse":p.update(tool_name="Bash",tool_input={"command":"rm -rf build" if sub=="delete" else "git status --short","shell":"bash"})
 if ev=="PostToolUse":
  if agent=="claude":p.update(tool_name="Write",tool_input={"file_path":str(path)})
  else:p.update(tool_name="apply_patch",tool_input={"command":"*** Begin Patch\n*** Add File: "+str(path)+"\n+x\n*** End Patch"})
 return p
def hooks():
 e=env("hooks");seed(e,100);m=mutex(e);p=Path(e["OBIT_HOME"])/"store.json";st=state(e);assert st["version"]==17
 for agent in ["claude","codex"]:
  for kind in ["PreToolUse/ordinary","PreToolUse/delete","PostToolUse/approved","PostToolUse/pending","SessionStart/startup","SessionStart/resume","Stop/dispatch"]:
   v=[];ev,sub=kind.split("/");seedstate=dict(st); path=repo/"debug.log";path.write_text("x");
   if sub=="pending":seedstate.pop("projects",None)
   for i in range(21):
    sid=agent+sub+str(i)+uuid.uuid4().hex[:5];p.write_text(json.dumps(seedstate),encoding="utf8")
    if sub in ["resume","dispatch"]:run("obit",["hook",agent],e,payload(agent,"SessionStart/startup",sid))
    d,out,err=run("obit",["hook",agent],e,payload(agent,kind,sid,path));v.append(d)
    if sub=="delete":assert b"deny" in out,out
    if sub=="ordinary":assert not out,out
    if sub=="approved":assert len(state(e)["items"])==101,(state(e).keys(),out)
    if sub=="pending":assert len(state(e)["proposals"])==1 and b"approve" in out,out
    if sub in ["resume","startup"]:assert any(sid in f.name for f in (p.parent/"sessions").glob("*.json")),(sid,out)
    if (p.parent/"hook.log").exists():raise RuntimeError((p.parent/"hook.log").read_text()[-800:])
   show(agent+"/"+kind,v)
def conc():
 for k in [4,8,16]:
  e=env("conc"+str(k));seed(e,1000 if "large" in sys.argv else 100);m=mutex(e);initial=len(state(e)["items"]);hooks=[];reads=[];groups=[];delayed=0
  for j in range(7):
   start=threading.Barrier(k+2);paths=[]
   for i in range(k):
    p=repo/(f"debug-{k}-{j}-{i}.log"); p.write_text("x"); paths.append(p)
   def job(i):
    start.wait();time.sleep(.005*(i%4));return run("obit",["hook","claude"],e,payload("claude","PostToolUse/approved",f"w{i}",paths[i]))[0]
   def reader():start.wait();time.sleep(.065);return run("obit",["list"],e)[0]
   with cf.ThreadPoolExecutor(k+1) as pool:
    fs=[pool.submit(job,i) for i in range(k)];rd=pool.submit(reader);t=time.perf_counter();start.wait();hooks.extend(f.result() for f in fs);groups.append((time.perf_counter()-t)*1000);reads.append(rd.result())
   expected=initial+(j+1)*k;delayed+=expected-len(state(e)["items"]);run("probe",["replay"],e);assert len(state(e)["items"])==expected,(expected,len(state(e)["items"]));time.sleep(.6)
  for n,v in [("hook",hooks),("group",groups),("list",reads)]:show(f"{k}/{n}",v)
  print("delayed",k,delayed,flush=True);time.sleep(2)
def cli():
 for n in [1000,10000]:
  e=env("big"+str(n));seed(e,n);print("seeded",n,flush=True);m=mutex(e)
  for a in [["list"],["stats"],["graveyard","--json"],["doctor","--json"],["reap"],["purge"]]:
   v=[]
   for i in range(7):
    d,out,err=run("obit",a,e,allow=a[0]=="doctor");v.append(d)
    if a[0]=="list":assert len(out.splitlines())==n+1
    if a[0]=="graveyard":assert len(json.loads(out)["burials"])==n
    if a[0]=="doctor":assert next(x for x in json.loads(out) if x["name"]=="store")["status"]=="PASS"
   show(str(n)+"/"+a[0],v)
  for command in ["phase","tick","medium"]:
   d,out,err=run("probe",[command],e); groups={}
   for line in out.splitlines():w=line.split();groups.setdefault(w[0].decode(),[]).append(float(w[1]))
   for key,v in groups.items():show(f"{n}/{key}",v)
  vals=[[],[]];
  for i in range(6):
   run("probe",["due"],e);d,out,_=run("obit",["reap","--json"],e);assert len(json.loads(out))==1,out;vals[0].append(d);run("probe",["arm"],e);d,out,_=run("obit",["purge","--json"],e);assert len(json.loads(out))==1,out;vals[1].append(d);run("probe",["resetone"],e)
  for i,key in enumerate(["reap-one","purge-one"]):show(f"{n}/{key}",vals[i])
def cost():
 e=env("cost");seed(e,100);p=payload("claude","PreToolUse/ordinary","s");seq=["obit","fast","empty"]
 if (B/"lazy.exe").exists():seq.append("lazy")
 vs={k:[] for k in seq};init={}
 for i in range(31):
  for ex in seq[i%len(seq):]+seq[:i%len(seq)]:vs[ex].append(run(ex,["hook","claude"],e,p)[0])
  _,_,err=run("obit",["hook","claude"],dict(e,GODEBUG="inittrace=1"),p); rows={k.decode():float(v) for k,v in re.findall(rb"init (\S+) @[^ ]+ ms, ([\d.]+) ms clock",err)};assert len(rows)>70;init.setdefault("all-init",[]).append(sum(rows.values()));
  for k,v in rows.items():init.setdefault(k,[]).append(v)
 for k,v in vs.items():show(k,v)
 for k,v in sorted(init.items(),key=lambda p:-ss.median(p[1]))[:12]:show(k,v)
if not (repo/".git").is_dir():sp.run(["git","init","-q",str(repo)],env=base,check=True)
print("OS",platform.platform(),"CPU",os.cpu_count(),"tmp",R,flush=True)
if __name__=="__main__":globals()[sys.argv[1]]()
