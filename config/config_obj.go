package config

import (
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/util/gconv"
	log "github.com/sirupsen/logrus"
)

type ConfObj struct {
	Supervisord    map[string]string
	InetHttpServer map[string]string
	UnixHttpServer map[string]string
	Supervisorctl  map[string]string
	Programs       map[string](map[string]interface{})
	Groups         map[string](map[string]interface{})
	EventListeners map[string](map[string]string)
}

func (c *Config) SetConfigObj(config_obj interface{}) error {
	if config_obj != nil {
		c.configObj = new(ConfObj)
		gconv.Struct(config_obj, c.configObj)
		fmt.Println("config obj is:", c.configObj)
	}
	return nil
}

func obj_copy(obj map[string]interface{}) map[string]interface{} {
	if obj == nil {
		return nil
	}
	n_obj := make(map[string]interface{})
	for k, v := range obj {
		n_obj[k] = v
	}
	return n_obj
}

func (c *Config) LoadObj() ([]string, error) {
	if c.configObj.Supervisord != nil {
		entry := new(Entry)
		entry.keyValues = c.configObj.Supervisord
		entry.Name = "supervisord"
		c.entries[entry.Name] = entry
	}
	if c.configObj.InetHttpServer != nil {
		entry := new(Entry)
		entry.keyValues = c.configObj.InetHttpServer
		entry.Name = "inet_http_server"
		c.entries[entry.Name] = entry
	}
	if c.configObj.UnixHttpServer != nil {
		entry := new(Entry)
		entry.keyValues = c.configObj.UnixHttpServer
		entry.Name = "unix_http_server"
		c.entries[entry.Name] = entry
	}
	if c.configObj.Supervisorctl != nil {
		entry := new(Entry)
		entry.keyValues = c.configObj.Supervisorctl
		entry.Name = "supervisorctl"
		c.entries[entry.Name] = entry
	}
	if c.configObj.Groups != nil {
		for group_name, group_conf := range c.configObj.Groups {
			entry := new(Entry)
			group_name_with_pref := "group:" + group_name
			entry.Name = group_name_with_pref
			entry.keyValues = gconv.MapStrStr(group_conf)
			if arr, ok := group_conf["programs"].([]interface{}); ok {
				progs_str := ""
				for _, item := range arr {
					proc_name, _ := item.(string)
					if progs_str != "" {
						progs_str = progs_str + ","
					}
					c.ProgramGroup.Add(group_name, proc_name)
				}
				entry.keyValues["programs"] = progs_str
			} else if str, ok := group_conf["programs"].(string); ok {
				arr := strings.Split(str, ",")
				for _, proc_name := range arr {
					proc_name = strings.TrimSpace(proc_name)
					c.ProgramGroup.Add(group_name, proc_name)
				}
			}
			c.entries[group_name_with_pref] = entry
		}
	}
	loaded_progs := make([]string, 0)
	if c.configObj.Programs != nil {
		default_conf, _ := c.configObj.Programs["default"]
		for name, conf := range c.configObj.Programs {
			// get the number of processes
			numProcs := gconv.Int(conf["numprocs"])
			if numProcs <= 0 {
				numProcs = 1
			}
			programName := name
			originalProcName := programName
			procName, ok := conf["process_name"].(string)
			if ok {
				originalProcName = procName
			}
			if m, ok := conf["environment"].(map[string]interface{}); ok {
				env_str := ""
				for k, v := range m {
					if env_str != "" {
						env_str = env_str + ","
					}
					env_str = env_str + k + "=\"" + gconv.String(v) + "\""
				}
				conf["environment"] = env_str
			}

			originalCmd, _ := conf["command"].(string)
			for i := 1; i <= numProcs; i++ {
				entry := new(Entry)
				if numProcs == 1 {
					entry.keyValues = gconv.MapStrStrDeep(conf)
				} else {
					entry.keyValues = gconv.MapStrStrDeep(obj_copy(conf))
				}
				if default_conf != nil {
					for k, v := range default_conf {
						if _, ok := entry.keyValues[k]; !ok {
							entry.keyValues[k] = gconv.String(v)
						}
					}
				}

				envs := NewStringExpression("program_name", programName,
					"process_num", fmt.Sprintf("%d", i),
					"group_name", c.ProgramGroup.GetGroup(programName, programName),
					"here", c.GetConfigFileDir())
				cmd, err := envs.Eval(originalCmd)
				if err != nil {
					log.WithFields(log.Fields{
						log.ErrorKey: err,
						"program":    programName,
					}).Error("get envs failed")
					continue
				}
				entry.keyValues["command"] = cmd

				procName := originalProcName
				if numProcs > 1 {
					if strings.Index(procName, "%(process_num)") >= 0 {
						procName, err = envs.Eval(originalProcName)
						if err != nil {
							log.WithFields(log.Fields{
								log.ErrorKey: err,
								"program":    programName,
							}).Error("get envs failed")
							continue
						}
					} else {
						procName = procName + "_" + fmt.Sprintf("%d", i)
					}
				}
				proc_name_env := "PROC_NAME=\"" + name + "\",INST_NAME=\"" + procName + "\""
				if env_vl, ok := entry.keyValues["environment"]; ok && env_vl != "" {
					entry.keyValues["environment"] = env_vl + "," + proc_name_env
				} else {
					entry.keyValues["environment"] = proc_name_env
				}

				entry.keyValues["process_name"] = procName
				entry.keyValues["numprocs_start"] = fmt.Sprintf("%d", i-1)
				entry.keyValues["process_num"] = fmt.Sprintf("%d", i)

				entry.Name = "program:" + procName
				loaded_progs = append(loaded_progs, procName)
				entry.Group = c.ProgramGroup.GetGroup(programName, programName)
				c.entries[procName] = entry
			}
		}
	}
	return loaded_progs, nil
}

func (c *Config) Print() {
	fmt.Println("config entries:")
	for k, v := range c.entries {
		fmt.Println("  key:", k)
		fmt.Println("  value.ConfigDir:", v.ConfigDir)
		fmt.Println("  value.Group:", v.Group)
		fmt.Println("  value.Name:", v.Name)
		fmt.Println("  value.KeyValues")
		for kk, vv := range v.keyValues {
			fmt.Println("    key:", kk, ",value:", vv)
		}
	}
	fmt.Println("config groups:")
	for k, v := range c.ProgramGroup.processGroup {
		fmt.Println("  key:", k, ", value:", v)
	}
}
