package main

import (
        "fmt"
        "sync"
        "github.com/gzsierra/go-lxc"
        "log"
        //"gopkg.in/cheggaaa/pb.v1"
        "time"
        "os/exec"
        "strconv"
)

type backup struct {
  container     *lxc.Container
  folder        string
  compressName  string
  scpHost       string
}

var (
      containers    []lxc.Container
      weekday       time.Weekday
      year          int
      month         time.Month
      day           int

      wg            sync.WaitGroup
)

const(
      backupDay     = 1 // 1 = monday, 2 = tuesday, ...
      lxcPath       = "/var/lib/lxc"
      lxcSnapPath   = "/var/lib/lxcsnaps"

      backupHost    = "IP_REMOTE_HOST"
      host          = "USERNAME@" + backupHost
      backupFolder  = "BACKUP_FOLDER"
      hostFolder    = host + ":" + backupFolder
)

func errPrint(err error)  {
  if err != nil {
    fmt.Println(err.Error())
  }
}

/*
`` * From lxc host, we launch update in a specific container
 */
func (b backup) backupC()  {
  // defer wg.Done() -> plan change
  c := b.container

  // GET SNAPSHOT
  snap, err := c.Snapshots()
  errPrint(err)

  fmt.Println(c.Name(), " : ", len(snap))
  // CREATE SNAPSHOT (MUST NOT BE OVER 7)
  if len(snap) < 7 {
    fmt.Println("Not enough snap ...")
    b.createSnap()

    // Done with the container
    wg.Done()

  } else {
    fmt.Println("Enough now!")
    b.compressShip()
  }

  // cdone++
}

func (b backup) compressShip()  {
  // Before exporting to external storage, we update for a newer version of the
  // container. We only keep 7 snapshots
  b.deleteSnap()
  b.createSnap()

  if weekday == backupDay {
    fmt.Println(weekday, " PASS")
    b.exportSnap()
  } else {
    fmt.Println("Hum something wrong : ",weekday,
                " ", b.container.Name())
    wg.Done()
  }
}

// LESS VERBOSE!!!
func (b backup) createSnap()  {
  c := b.container
  fmt.Println("Creating Snapshot - ", c.Name())

  // Here we stop the container before starting
  b.running()

  _, err := c.CreateSnapshot()
  errPrint(err)
  // Once we finish, we restart the container
  b.running()
}

func (b backup) deleteSnap()  {
  c := b.container
  fmt.Println("Deleting oldest Snapshot - ", c.Name())

  // We delete the oldest snapshot, only to keep max 7 snapshots
  list, _ := c.Snapshots()
  c.DestroySnapshot(list[0])
}

func (b backup) exportSnap()  {
  /////////////////////////
  fmt.Println("Create remote folder if not exist - ", b.container.Name())
  arg := "mkdir -p " + backupFolder + b.container.Name()
  _, err := exec.Command("ssh", host, arg).Output()
  errPrint(err)

  /////////////////////////
  fmt.Println("COMPRESS - ", b.container.Name())
  _, err = exec.Command("tar", "-czf", b.compressName, b.folder).Output()
  errPrint(err)

  /////////////////////////
  fmt.Println("SHIP - ", b.container.Name())
  // Since this doesn't use a lot of IO, it can be process by another process
  go func ()  {
    _, err = exec.Command("scp", b.compressName, b.scpHost).Output()
    errPrint(err)

    fmt.Println("DELETE Archive - ", b.container.Name())
    wg.Done()
  }()
}

func (b backup) running(){
  c := b.container
  run := exec.Command("lxc-start", "-d", "-n", c.Name())
  if c.Running() {
    run = exec.Command("lxc-stop", "-n", c.Name())
  }

  errPrint(run.Run())
}


//****************************************************************************//


/*
 * Launch routine to update containers
 */
func main() {
  // Seting up the time for easy access
  time := time.Now()
  weekday = time.Weekday()
  _, month, day = time.Date()

  // GET CONTAINERS
  getC()
}

// GET CONTAINERS
func getC()  {
  containers = lxc.ActiveContainers(lxcPath)
  wg.Add(len(containers))

  for i := range containers {
    log.Printf("%s\n", containers[i].Name())
    startRoutine(containers[i])
  }

  // Wait for everything to be done before exit
  wg.Wait()
}

func startRoutine(c lxc.Container){
  b := backup{
        container: &c,
        folder: lxcSnapPath + "/" + c.Name(),
        compressName: c.Name() + "-" + month.String() + strconv.Itoa(day) + ".tar.gz",
      }
  b.scpHost = hostFolder + c.Name() + "/" + b.compressName
  // go b.backupC()
  b.backupC()
}
