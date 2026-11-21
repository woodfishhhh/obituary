from detail import *
def qpc():
 root=B.parents[2];f=root/"internal/hook/helpers_test.go";src=f.read_text().replace(chr(34)+"bytes"+chr(34),chr(34)+"bytes"+chr(34)+"\n\t"+chr(34)+"golang.org/x/sys/windows"+chr(34)+"\n\t"+chr(34)+"unsafe"+chr(34));src=src.replace("started := time.Now()","started := auditQPC()").replace("elapsed := time.Since(started)","elapsed := time.Duration((auditQPC()-started)*1e9/auditFreq)");src+="\nvar auditK=windows.NewLazySystemDLL(\"kernel32.dll\")\nvar auditProc=auditK.NewProc(\"QueryPerformanceCounter\")\nvar auditFreq=func()int64{var n int64;auditK.NewProc(\"QueryPerformanceFrequency\").Call(uintptr(unsafe.Pointer(&n)));return n}()\nfunc auditQPC()int64{var n int64;auditProc.Call(uintptr(unsafe.Pointer(&n)));return n}\n";mod=B/"helpers_test.go";mod.write_text(src);over=B/"qpc.json";over.write_text(json.dumps({"Replace":{str(f):str(mod)}}));rootcache=sp.check_output(["go","env","GOCACHE"],text=True).strip();cache=B/"cache";layer=dict(os.environ,GOCACHE=str(cache),GOCACHEPROG=sp.list2cmdline([str(B/"cachehost.exe"),"obit-test-gocacheprog",str(cache),rootcache]),TEMP=str(R),TMP=str(R));sp.run(["go","test","-overlay="+str(over),"-run","^NoTest$","-bench","Benchmark(Claude|Codex)Hook/PreToolUse/ordinary","-benchtime=101x","./internal/hook"],cwd=root,env=layer,check=True)
def resident():
 vals=[[],[],[]];
 for i in range(7):
  e=env("idle-"+uuid.uuid4().hex);seed(e,0);p=sp.Popen([str(B/"obitd.exe"),"--tray=false"],env=e,cwd=repo,creationflags=sp.CREATE_NO_WINDOW);stop=Path(e["OBIT_HOME"])/"obitd.stop"
  try:
   time.sleep(.8);assert p.poll() is None;before=counters(p);a=sysread();time.sleep(1.8);assert p.poll() is None;after=counters(p);b=sysread();assert after[1]>4;vals[0].append(after[1]);vals[1].append(after[0]-before[0]);vals[2].append(100*(1-(b[0]-a[0])/(b[1]-a[1])))
  finally:stop.touch();p.wait(timeout=8)
 for k,v in zip(["WS-MiB","CPU-ms-1.8s","hostCPU-pct"],vals):show(k,v)
def trace():
 e=env("trace"+uuid.uuid4().hex);seed(e,100);m=mutex(e);root=Path(e["OBIT_HOME"]);store=root/"store.json";original=state(e);path=repo/"debug.log";path.write_text("x")
 for kind in ["PostToolUse/approved","PostToolUse/pending","SessionStart/startup","SessionStart/resume"]:
  num=[];inside=[];external=[];cmds=[]
  for i in range(9):
   st=dict(original);sid=uuid.uuid4().hex
   if kind.endswith("pending"):st.pop("projects",None)
   if kind.endswith("resume"):run("obit",["hook","claude"],e,payload("claude","SessionStart/startup",sid))
   store.write_text(json.dumps(st));tracefile=root/(uuid.uuid4().hex+".jsonl");d,out,err=run("obit",["hook","claude"],dict(e,GIT_TRACE2_EVENT=str(tracefile)),payload("claude",kind,sid,path));evs=[json.loads(l) for l in tracefile.read_bytes().splitlines()] if tracefile.exists() else [];num.append(sum(x["event"]=="start" for x in evs));inside.append(1000*sum(x["t_abs"] for x in evs if x["event"]=="exit"));external.append(d);cmds=[x["argv"][1:] for x in evs if x["event"]=="start"]
  for tag,v in [("child-count",num),("child-internal-ms",inside),("wall",external)]:show(kind+"/"+tag,v)
  print("git:",cmds)
def fence():
 e=env("fence-"+uuid.uuid4().hex);p=Path(e["OBIT_HOME"])/"store.json";p.write_text(json.dumps({"version":18,"storage":"sqlite","database":"store.db"}));before=p.read_bytes();r=sp.run([str(B/"obit.exe"),"list","--json"],env=e,cwd=repo,capture_output=True);assert r.returncode!=0 and b"newer" in r.stderr and before==p.read_bytes();print(r.returncode,r.stderr.decode().strip())
def locks():
 for n in [1000,10000]:
  e=env("big"+str(n));_,out,_=run("medium",["medium"],e);a=[float(l.split()[1]) for l in out.splitlines()];show(str(n),a)
if __name__=="__main__":globals()[sys.argv[1]]()
