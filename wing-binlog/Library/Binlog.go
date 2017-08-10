package Library

import (
	//"fmt"
	_ "github.com/go-sql-driver/mysql"
)

type Binlog struct {

}


/**
     * 获取所有的logs
     *
     * @return array
     */
func (_binlog Binlog) GetLogs()   map[int]map[string]interface {} {
	sql  := "show binary logs"
	//if (WING_DEBUG) {
	//	fmt.Println(sql + "\r\n")
	//}
	pdo := Pdo{"root", "123456", "xl"}

	return pdo.Query(sql)//$this->db_handler->query($sql);
}
//
//public function getFormat()
//{
//$sql  = 'select @@binlog_format';
//if (WING_DEBUG)
//echo $sql, "\r\n";
//
//$data = $this->db_handler->row($sql);
//return strtolower($data["@@binlog_format"]);
//}
//
///**
// * 获取当前正在使用的binglog日志文件信息
// *
// * @return array 一维
// *    array(5) {
// *           ["File"] => string(16) "mysql-bin.000005"
// *           ["Position"] => int(8840)
// *           ["Binlog_Do_DB"] => string(0) ""
// *           ["Binlog_Ignore_DB"] => string(0) ""
// *           ["Executed_Gtid_Set"] => string(0) ""
// *     }
// */
//public function getCurrentLogInfo()
//{
//$sql  = 'show master status';
//if (WING_DEBUG)
//echo $sql, "\r\n";
//
//$data = $this->db_handler->row($sql);
//return $data;
//}
//
///**
// * 获取所有的binlog文件
// *
// * @return array
// */
//public function getFiles()
//{
//$logs  = $this->getLogs();
//$sql   = 'select @@log_bin_basename';
//if (WING_DEBUG)
//echo $sql, "\r\n";
//
//$data  = $this->db_handler->row($sql);
//$path  = pathinfo($data["@@log_bin_basename"],PATHINFO_DIRNAME);
//$files = [];
//
//foreach ($logs as $line) {
//$files[] = $path.DIRECTORY_SEPARATOR.$line["Log_name"];
//}
//
//return $files;
//}
//
///**
// * 获取当前正在使用的binlog文件路径
// *
// * @return string
// */
//private $start_getCurrentLogFile = null;
//public function getCurrentLogFile()
//{
//if ($this->start_getCurrentLogFile == null) {
//$this->start_getCurrentLogFile = time();
//}
//if ($this->current_binlog_file != null ) {
//if ((time() - $this->start_getCurrentLogFile) < 5) {
//return $this->current_binlog_file;
//} else {
//$this->start_getCurrentLogFile = time();
//}
//}
//
//$sql  = 'select @@log_bin_basename';
//if (WING_DEBUG)
//echo $sql, "\r\n";
//
//$data = $this->db_handler->row($sql);
//
//if (!isset($data["@@log_bin_basename"]))
//return null;
//
//$path = pathinfo($data["@@log_bin_basename"],PATHINFO_DIRNAME);
//$info = $this->getCurrentLogInfo();
//
//if (!isset($info["File"]))
//return null;
//
//$path = $path . DIRECTORY_SEPARATOR . $info["File"];
////        if (file_exists($path))
////            $this->cache->set($key, $path, 3);
////        else
////            $this->cache->del($key);
//
//$this->current_binlog_file = $path;
//return $path;
//}
//
///**
// * 检测是否已开启binlog功能
// *
// * @return bool
// */
//public function isOpen()
//{
//$sql  = 'select @@sql_log_bin';
//if (WING_DEBUG)
//echo $sql, "\r\n";
//
//$data = $this->db_handler->row($sql);
//return isset($data["@@sql_log_bin"]) && $data["@@sql_log_bin"] == 1;
//}
//
//
///**
// * 设置存储最后操作的binlog名称--游标，请勿删除mysql.last
// *
// * @param string $binlog
// */
//public function setLastBinLog($binlog)
//{
//if (WING_DEBUG)
//echo "保存最后的读取的binlog文件：",$binlog,"\r\n";
//return $this->cache_handler->set("mysql.last", $binlog);
//}
//
///**
// * 获取最后操作的binlog文件名称
// *
// * @return string
// */
//public function getLastBinLog()
//{
//if (WING_DEBUG)
//echo "获取最后读取的binlog\r\n";
//return $this->cache_handler->get("mysql.last");
//}
//
///**
// * 设置最后的读取位置--游标，请勿删除mysql.pos
// *
// * @param int $start_pos
// * @param int $end_pos
// * @return bool
// */
//public function setLastPosition($start_pos,$end_pos)
//{
//if (WING_DEBUG)
//echo "保存最后读取为位置：", $start_pos,":",$end_pos,"\r\n";
//return $this->cache_handler->set("mysql.pos", [$start_pos,$end_pos]);
//}
//
///**
// * 获取最后的读取位置
// *
// * @return array
// */
//public function getLastPosition()
//{
//if (WING_DEBUG)
//echo "获取最后读取的位置\r\n";
//return $this->cache_handler->get("mysql.pos");
//}
//
///**
// * 获取binlog事件，请只在意第一第二个参数
// *
// * @return array
// */
//public function getEvents($current_binlog,$last_end_pos, $limit = 10000)
//{
//if (!$last_end_pos)
//$last_end_pos = 0;
//
//$sql   = 'show binlog events in "' . $current_binlog . '" from ' . $last_end_pos.' limit '.$limit;
//$datas = $this->db_handler->query($sql);
//
//if ($datas) {
//if (WING_DEBUG)
//echo $sql,"\r\n";
//}
//
//return $datas;
//}
//
///**
// * 获取session元数据--直接存储于cache_file
// *
// * @return string 缓存文件路径
// */
//public function getSessions($worker, $start_pos, $end_pos)
//{
////当前使用的binlog文件路径
//$current_binlog_file = $this->getCurrentLogFile();
//if (!$current_binlog_file) {
//$error = "get current binlog path error => ".$current_binlog_file;
//if (WING_DEBUG)
//echo $error,"\r\n";
//// Context::instance()->logger->error($error);
//}
//
//$str1 = md5(rand(0,999999));
//$str2 = md5(rand(0,999999));
//$str3 = md5(rand(0,999999));
//$dir = HOME."/cache/binfile/".$worker;
//(new WDir($dir))->mkdir();
//
//$file_name = time().
//substr($str1,rand(0,strlen($str1)-16),8).
//substr($str2,rand(0,strlen($str2)-16),8).
//substr($str3,rand(0,strlen($str3)-16),8);
//
//$cache_file  = $dir."/lock__".$file_name;
//
//unset($str1,$str2,$str3);
//
////mysqlbinlog -uroot -proot -h127.0.0.1 -P3306 --read-from-remote-server mysql-bin.000001 --base64-output=decode-rows -v > 1
///*$command    = $this->mysqlbinlog .
//	" -u".$this->user.
//	" -p\"".$this->password."\"".
//	" -h".$this->host.
//	" -P".$this->port.
//	//" --read-from-remote-server".
//	" -R --base64-output=DECODE-ROWS -v". //-vv
//	" --start-position=" . $start_pos .
//	" --stop-position=" . $end_pos .
//	"  \"" . $current_binlog_file . "\" > ".$cache_file;
//*/
//// echo preg_replace("/\-p[\s\S]{1,}?\s/","-p****** ",$command,1),"\r\n";
//$command    =
//$this->mysqlbinlog .
//" --base64-output=DECODE-ROWS -v".
//" --start-position=" . $start_pos .
//" --stop-position=" . $end_pos . "  \"" . $current_binlog_file . "\" > ".$cache_file ;
//
//if (WING_DEBUG)
//echo $command,"\r\n";
//
//unset($current_binlog_file);
//$handle = popen($command,"r");
//if (!$handle) {
//pclose($handle);
//}
//
//if (!file_exists($cache_file)) {
//system($command);
//}
//if (!file_exists($cache_file)) {
//system($command);
//}
//if (!file_exists($cache_file)) {
//system($command);
//}
//if (!file_exists($cache_file)) {
//system($command);
//}
//if (!file_exists($cache_file)) {
//system($command);
//}
//
//if (file_exists($cache_file)) {
//rename($cache_file, $dir."/".$file_name);
//}
//
//if (file_exists($cache_file)) {
//rename($cache_file, $dir."/".$file_name);
//}
//
//if (file_exists($cache_file)) {
//rename($cache_file, $dir."/".$file_name);
//}
//
//unset($command);
//return $cache_file;
//}

//func (r Binlog) ToString() string {
//	fmt.Printf("类型是：%s\r\n", reflect.TypeOf(r.Data))
//	switch r.Data.(type) {
//	case string:
//		return string(r.Data.(string));
//	case []uint8:
//		return string(r.Data.([]byte))
//	case int:
//		return strconv.Itoa(int(r.Data.(int)))
//	case int64:
//		return strconv.FormatInt(int64(r.Data.(int64)),10)
//	case uint:
//		return strconv.Itoa(int(r.Data.(uint)))
//	}
//	return "";
//}
