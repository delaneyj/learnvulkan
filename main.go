package main

import (
	"log"
	"runtime"
	"sort"
	"strings"
	"unsafe"

	"github.com/pkg/errors"
	"github.com/vulkan-go/glfw/v3.3/glfw"
	vk "github.com/vulkan-go/vulkan"
)

const (
	width                  = 1280
	height                 = 720
	title                  = "Learn Vulkan"
	enableValidationLayers = true
)

var (
	validationLayerNames = []string{
		"VK_LAYER_LUNARG_standard_validation",
	}
)

func init() {
	// This is needed to arrange that main() runs on main thread.
	// See documentation for functions that are only allowed to be called from the main thread.
	runtime.LockOSThread()
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("Starting %s", title)
	defer log.Printf("Closing %s", title)

	app := HelloTriangleApplication{}
	if err := app.Run(); err != nil {
		log.Fatal(errors.Wrapf(err, "can't run %s", title))
	}

	log.Printf("Ran %s successfully", title)
}

type HelloTriangleApplication struct {
	window   *glfw.Window
	instance vk.Instance
	debug    vk.DebugReportCallback
}

func (app *HelloTriangleApplication) Run() error {
	defer app.cleanup()

	if err := app.initWindow(); err != nil {
		return errors.Wrap(err, "can't init window")
	}
	if err := app.initVulkan(); err != nil {
		return errors.Wrap(err, "can't init vulkan")
	}

	if err := app.mainLoop(); err != nil {
		return errors.Wrap(err, "can't init vulkan")
	}

	return nil
}

func (app *HelloTriangleApplication) initWindow() error {
	if err := glfw.Init(); err != nil {
		return errors.Wrap(err, "can't init GLFW")
	}

	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	glfw.WindowHint(glfw.Resizable, glfw.False)

	window, err := glfw.CreateWindow(width, height, title, nil, nil)
	if err != nil {
		return errors.Wrap(err, "can't create GLFW window")
	}
	app.window = window
	return nil
}

func (app *HelloTriangleApplication) initVulkan() error {
	procAddr := glfw.GetVulkanGetInstanceProcAddress()
	if procAddr == nil {
		return errors.New("GLFW instanceProcAddress is nil")
	}
	vk.SetGetInstanceProcAddr(procAddr)

	if err := vk.Init(); err != nil {
		return errors.Wrap(err, "can't vk.init()")
	}

	if err := app.createInstance(); err != nil {
		return errors.Wrap(err, "can't create vk instance")
	}

	if err := app.setupDebugCallback(); err != nil {
		return errors.Wrap(err, "can't create vk instance")
	}

	if _, err := app.pickPhysicalDevice(); err != nil {
		return errors.Wrap(err, "can't pick physical device")
	}

	return nil
}

func (app *HelloTriangleApplication) mainLoop() error {
	w := app.window
	// w.MakeContextCurrent()
	for !w.ShouldClose() {
		glfw.PollEvents()
		// w.SwapBuffers()

		if w.GetKey(glfw.KeyEscape) == glfw.Press {
			break
		}
	}
	return nil
}

func (app *HelloTriangleApplication) cleanup() {
	if enableValidationLayers && app.debug != nil && app.debug != vk.NullDebugReportCallback {
		vk.DestroyDebugReportCallback(app.instance, app.debug, nil)
	}

	if app.instance != nil {
		vk.DestroyInstance(app.instance, nil)
	}

	if app.window != nil {
		app.window.Destroy()
	}

	glfw.Terminate()
}

func (app *HelloTriangleApplication) createInstance() error {
	if enableValidationLayers {
		supported, err := app.checkValidationLayerSupport()
		if err != nil {
			return errors.Wrap(err, "can't check validation layers")
		}
		if !supported {
			return errors.New("validation layers requested but not available")
		}
	}

	appInfo := &vk.ApplicationInfo{
		SType:              vk.StructureTypeApplicationInfo,
		PApplicationName:   title,
		ApplicationVersion: vk.MakeVersion(1, 0, 0),
		PEngineName:        "Ingot",
		EngineVersion:      vk.MakeVersion(1, 0, 0),
		ApiVersion:         vk.ApiVersion11,
	}

	var availableInstanceExtensionsCount uint32
	if err := vk.Error(vk.EnumerateInstanceExtensionProperties("", &availableInstanceExtensionsCount, nil)); err != nil {
		return errors.Wrap(err, "can't enumerate instance extensions")
	}
	availableInstanceExtensions := make([]vk.ExtensionProperties, availableInstanceExtensionsCount)
	if err := vk.Error(vk.EnumerateInstanceExtensionProperties("", &availableInstanceExtensionsCount, availableInstanceExtensions)); err != nil {
		return errors.Wrap(err, "can't enumerate instance extensions")
	}

	log.Printf("Available extensions")
	for _, ex := range availableInstanceExtensions {
		ex.Deref()
		log.Printf(" > " + vk.ToString(ex.ExtensionName[:]))
	}

	requiredExtensions := app.requiredExtensions()
	log.Printf(
		"Attempting to create instance with %s extensions enabled",
		strings.Join(requiredExtensions, ","),
	)
	createInfo := &vk.InstanceCreateInfo{
		SType:                   vk.StructureTypeInstanceCreateInfo,
		PApplicationInfo:        appInfo,
		EnabledExtensionCount:   uint32(len(requiredExtensions)),
		PpEnabledExtensionNames: requiredExtensions,
	}

	var instance vk.Instance
	if err := vk.Error(vk.CreateInstance(createInfo, nil, &instance)); err != nil {
		return errors.Wrap(err, "can't create instance")
	}
	app.instance = instance
	return nil
}

func (app *HelloTriangleApplication) requiredExtensions() []string {
	requiredExtensions := app.window.GetRequiredInstanceExtensions()

	if enableValidationLayers {
		requiredExtensions = append(requiredExtensions, vk.ExtDebugReportExtensionName+"\x00")
	}

	return requiredExtensions
}

func (app *HelloTriangleApplication) checkValidationLayerSupport() (bool, error) {
	var layerCount uint32
	if err := vk.Error(vk.EnumerateInstanceLayerProperties(&layerCount, nil)); err != nil {
		return false, errors.Wrap(err, "can't get layer count")
	}
	availableLayers := make([]vk.LayerProperties, layerCount)

	if err := vk.Error(vk.EnumerateInstanceLayerProperties(&layerCount, availableLayers)); err != nil {
		return false, errors.Wrap(err, "can't get layers")
	}

	log.Print("Available validation layers")
	availableLayerNames := make([]string, len(availableLayers))
	for i, layer := range availableLayers {
		layer.Deref()
		name := vk.ToString(layer.LayerName[:])
		description := vk.ToString(layer.Description[:])
		log.Printf(" > %s <%s>", name, description)
		availableLayerNames[i] = name
	}

	for _, vln := range validationLayerNames {
		found := false
		for _, aln := range availableLayerNames {
			if vln == aln {
				log.Printf("%s is a supported validation layer", aln)
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}
	log.Print("All validation layers are supported")

	return true, nil
}

func (app *HelloTriangleApplication) setupDebugCallback() error {
	if !enableValidationLayers {
		return nil
	}

	flags := vk.DebugReportFlags(vk.DebugUtilsMessageSeverityVerboseBit | vk.DebugUtilsMessageSeverityWarningBit | vk.DebugUtilsMessageSeverityErrorBit)
	createInfo := &vk.DebugReportCallbackCreateInfo{
		SType:       vk.StructureTypeDebugUtilsMessengerCreateInfo,
		Flags:       flags,
		PfnCallback: debugCallback,
	}

	var debugReportCallback vk.DebugReportCallback
	if err := vk.Error(vk.CreateDebugReportCallback(app.instance, createInfo, nil, &debugReportCallback)); err != nil {
		return errors.Wrap(err, "can't create debug report")
	}
	app.debug = debugReportCallback
	return nil
}

func debugCallback(flags vk.DebugReportFlags, objectType vk.DebugReportObjectType,
	object uint64, location uint, messageCode int32, pLayerPrefix string,
	pMessage string, pUserData unsafe.Pointer) vk.Bool32 {

	switch {
	case flags&vk.DebugReportFlags(vk.DebugReportErrorBit) != 0:
		log.Printf("[ERROR %d] %s on layer %s", messageCode, pMessage, pLayerPrefix)
	case flags&vk.DebugReportFlags(vk.DebugReportWarningBit) != 0:
		log.Printf("[WARN %d] %s on layer %s", messageCode, pMessage, pLayerPrefix)
	default:
		log.Printf("[WARN] unknown debug message %d (layer %s)", messageCode, pLayerPrefix)
	}
	return vk.Bool32(vk.False)
}

func (app *HelloTriangleApplication) pickPhysicalDevice() (physicalDevice vk.PhysicalDevice, err error) {

	var deviceCount uint32
	if err = vk.Error(vk.EnumeratePhysicalDevices(app.instance, &deviceCount, nil)); err != nil {
		err = errors.Wrap(err, "can't get physical device count")
		return
	}

	devices := make([]vk.PhysicalDevice, deviceCount)
	if err = vk.Error(vk.EnumeratePhysicalDevices(app.instance, &deviceCount, devices)); err != nil {
		err = errors.Wrap(err, "can't get physical device count")
		return
	}

	if len(devices) == 0 {
		err = errors.New("no phyical device detected")
		return
	}

	type deviceScore struct {
		Device vk.PhysicalDevice
		Name   string
		Score  uint32
	}
	candidates := make([]deviceScore, len(devices))

	for i, d := range devices {
		var score uint32

		// Discrete GPUs have a significant performance advantage
		var properties vk.PhysicalDeviceProperties
		vk.GetPhysicalDeviceProperties(d, &properties)
		properties.Deref()
		if isDiscrete := properties.DeviceType == vk.PhysicalDeviceTypeDiscreteGpu; isDiscrete {
			score += 1000
		}

		var features vk.PhysicalDeviceFeatures
		vk.GetPhysicalDeviceFeatures(d, &features)
		features.Deref()

		// Maximum possible size of textures affects graphics quality
		score += properties.Limits.MaxImageDimension2D

		// Application can't function without geometry shaders
		if hasGeometryShader := features.GeometryShader.B(); hasGeometryShader {
			score = 0
		}

		candidates[i] = deviceScore{
			Device: d,
			Name:   vk.ToString(properties.DeviceName[:]),
			Score:  score,
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		return a.Score > b.Score
	})

	chosen := candidates[0]
	physicalDevice = chosen.Device

	if physicalDevice == nil {
		err = errors.New("failed to find suitable GPU")
		return
	}

	log.Printf("Selecting physical device '%s'", chosen.Name)
	return
}
