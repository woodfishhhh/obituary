from driver import *
from ctypes import wintypes as wt
def build():
 root=B.parents[2];target=root/"internal/i18n/messages.go";text=target.read_text();text=text.replace(chr(34)+"embed"+chr(34),chr(34)+"embed"+chr(34)+"\n\t"+chr(34)+"sync"+chr(34)).replace("var messageBundle = func()", "var messageBundle = sync.OnceValue(func()").replace("return bundle\n}()","return bundle\n})").replace("NewLocalizer(messageBundle,","NewLocalizer(messageBundle(),");assert "sync.OnceValue" in text and text.count("messageBundle()") == 2
 (B/"messages.go").write_text(text); mapping={str(target):str(B/"messages.go")};(B/"overlay.json").write_text(json.dumps({"Replace":mapping}));
 rootcache=sp.check_output(["go","env","GOCACHE"],text=True).strip();cache=B/"cache";cache.mkdir(exist_ok=True);layer=dict(os.environ,GOCACHE=str(cache),GOCACHEPROG=sp.list2cmdline([str(B/"cachehost.exe"),"obit-test-gocacheprog",str(cache),rootcache]));args=["go","build","-overlay="+str(B/"overlay.json"),"-ldflags=-s -w","-o",str(B/"lazy.exe"),str(B/"fast.go")];sp.run(args,cwd=root,env=layer,check=True)
def sysread():
 a,b,c=(ct.c_uint64() for _ in range(3));assert K.GetSystemTimes(ct.byref(a),ct.byref(b),ct.byref(c));return a.value,b.value+c.value
def counters(p):
 h=ct.c_void_p(int(p._handle));a,b,c,d=(ct.c_uint64() for _ in range(4));assert K.GetProcessTimes(h,ct.byref(a),ct.byref(b),ct.byref(c),ct.byref(d));m=(ct.c_size_t*12)();ct.cast(m,ct.POINTER(ct.c_uint32))[0]=ct.sizeof(m);assert ct.windll.psapi.GetProcessMemoryInfo(h,m,ct.sizeof(m));return (c.value+d.value)/10000,m[2]/1048576,m[9]/1048576
def res():
 for binary in ["obitd","probe"]:
  mem=[];cpu=[];load=[];burst=[];steps=[]
  for i in range(5):
   e=env("res"+binary+str(i));seed(e,0);p=sp.Popen([str(B/(binary+".exe")),"--tray=false" if binary=="obitd" else "resident"],env=e,cwd=repo,stdin=sp.PIPE,stdout=sp.PIPE,stderr=sp.PIPE,creationflags=sp.CREATE_NO_WINDOW)
   try:
    if binary=="probe":assert p.stdout.readline().strip()==b"ready"
    time.sleep(.6);before=counters(p);a=sysread();time.sleep(1.5);after=counters(p);b=sysread();mem.append(after[1]);cpu.append(after[0]-before[0]);load.append((1-(b[0]-a[0])/(b[1]-a[1]))*100)
    if binary=="probe":
     initial=state(e)["nextId"];o,err=p.communicate(b"burst\n",timeout=45);assert p.returncode==0,(o,err);ls=o.splitlines();assert ls[0]==b"idle 0",ls;assert int(ls[2].split()[1])==initial+1000,ls;burst.append(float(ls[1].split()[1]));steps.append(float(ls[1].split()[2]))
    else:(Path(e["OBIT_HOME"])/"obitd.stop").touch();p.wait(8)
   finally:
    if p.poll() is None:p.kill();p.wait()
  for k,v in [("working-set-MiB",mem),("cpu-ms-1.5s",cpu),("host-cpu-pct",load),("burst-ms",burst),("steps",steps)]:
   if v:show(binary+"/"+k,v)
def request(pipe,key,id,drop=False):
 for attempt in range(20):
  try:f=open(pipe,"r+b",buffering=0);break
  except OSError:K.WaitNamedPipeW(pipe,100)
 else:raise RuntimeError("pipe busy")
 with f:
  f.write(json.dumps(dict(Key=key,ID=id,Drop=drop)).encode()+b"\n");return None if drop else int(f.readline())
def sql():
 for mode in ["single","batch"]:
  db=R/(uuid.uuid4().hex+".db");pipe=chr(92)*2+"."+chr(92)+"pipe"+chr(92)+"obit-audit-6926-"+uuid.uuid4().hex;e=env("sql")
  def server():
   with open(B/("commit-"+mode+".txt"),"ab") as log:p=sp.Popen([str(B/"sql.exe"),str(db),pipe,mode],env=e,cwd=repo,stdout=sp.PIPE,stderr=log,creationflags=sp.CREATE_NO_WINDOW)
   line=p.stdout.readline();assert line.strip()==b"ready",line;return p
  p=server();r=sqlite3.connect(db.as_uri()+"?mode=ro",uri=True,isolation_level=None);count=0
  try:
   for n in [1,4,8,16]:
    times=[];tot=[];reads=[]
    with cf.ThreadPoolExecutor(n+1) as pool:
     for j in range(21):
      bar=threading.Barrier(n+1)
      def job(i):
       bar.wait();t=time.perf_counter_ns();key=f"{n}/{i}/{j}";assert request(pipe,key,i+1)>0;return (time.perf_counter_ns()-t)/1e6
      fut=[pool.submit(job,i) for i in range(n)];t=time.perf_counter_ns();bar.wait();u=time.perf_counter_ns();r.execute("SELECT rev FROM items WHERE id=1").fetchone();reads.append((time.perf_counter_ns()-u)/1e6);times.extend(f.result() for f in fut);tot.append((time.perf_counter_ns()-t)/1e6);count+=n
    for k,v in [("ack",times),("group",tot),("read-one",reads)]:show(f"{mode}/{n}/{k}",v)
   assert r.execute("select sum(rev) from items").fetchone()[0]==count;request(pipe,"lost",1,True)
   for i in range(100):
    if r.execute("select count(*) from receipts where k=?",("lost",)).fetchone()[0]:break
    time.sleep(.005)
   assert i<99;p.kill();p.wait();p=server();res=request(pipe,"lost",1);assert res==r.execute("select rev from items where id=1").fetchone()[0] and r.execute("select sum(rev) from items").fetchone()[0]==count+1;print("ack lost/restart/retry OK",mode,count,flush=True)
  finally:r.close();p.kill();p.wait()
if __name__=="__main__":globals()[sys.argv[1]]()
