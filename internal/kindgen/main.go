package main
import (
 "os"
"bytes"
"gopkg.in/yaml.v3"
 "github.com/jasondeutsch/watts/internal/manifest"
 "github.com/jasondeutsch/watts/internal/render"
)
func main(){
 for _,dir:=range []string{"resource-templates","resources/examples/go_generalist"}{
  path:=dir+"/values.yaml"
  data,err:=os.ReadFile(path);if err!=nil{panic(err)}
  var values map[string]any;if err=yaml.Unmarshal(data,&values);err!=nil{panic(err)}
  for name,value:=range values["config_maps"].(map[string]any){
   spec:=value.(map[string]any)
   if _,ok:=spec["data"];!ok{values["config_maps"].(map[string]any)[name]=map[string]any{"data":spec}}
  }
  var out bytes.Buffer;e:=yaml.NewEncoder(&out);e.SetIndent(2);if err=e.Encode(values);err!=nil{panic(err)}
  if err=os.WriteFile(path,out.Bytes(),0644);err!=nil{panic(err)}
 }

 root,err:=os.OpenRoot("resources/examples/go_generalist");if err!=nil{panic(err)};defer root.Close()
 result,err:=render.Render(root.FS());if err!=nil{panic(err)}
 var sources [][]byte
 for _,file:=range result.Files{sources=append(sources,file.Data)}
 data,err:=manifest.JoinDocuments(sources...);if err!=nil{panic(err)}
 if _,err=manifest.CompileWorkflow(data);err!=nil{panic(err)}
 if err=os.WriteFile("resources/examples/go_generalist/workflows/go-generalist.yaml",data,0644);err!=nil{panic(err)}
}
